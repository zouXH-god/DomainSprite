package nodecontrol

import (
	nodev1 "DDNSServer/api/node/v1"
	"DDNSServer/db"
	"DDNSServer/models"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const ProtocolVersion = "1"

type streamRef struct {
	stream nodev1.NodeService_ConnectServer
	send   sync.Mutex
}
type Hub struct {
	sync.RWMutex
	streams map[uint64]*streamRef
}

func NewHub() *Hub { return &Hub{streams: map[uint64]*streamRef{}} }

var DefaultHub = NewHub()

func (h *Hub) Send(nodeID uint64, msg *nodev1.ControllerMessage) error {
	h.RLock()
	s := h.streams[nodeID]
	h.RUnlock()
	if s == nil {
		return errors.New("节点未连接")
	}
	s.send.Lock()
	defer s.send.Unlock()
	return s.stream.Send(msg)
}

type Server struct {
	nodev1.UnimplementedNodeServiceServer
	hub      *Hub
	caCert   *x509.Certificate
	caKey    ed25519.PrivateKey
	caPEM    []byte
	grpc     *grpc.Server
	listener net.Listener
}

func NewServer(h *Hub) (*Server, error) {
	cert, key, certPEM, err := loadOrCreateCA()
	if err != nil {
		return nil, err
	}
	return &Server{hub: h, caCert: cert, caKey: key, caPEM: certPEM}, nil
}
func (s *Server) Start(cfg models.GRPCConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
		return errors.New("gRPC 已启用，但 TLSCertFile 或 TLSKeyFile 未配置")
	}
	pair, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return fmt.Errorf("加载 gRPC TLS: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(s.caPEM)
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, ClientCAs: pool, ClientAuth: tls.VerifyClientCertIfGiven}
	lis, err := net.Listen("tcp", net.JoinHostPort(cfg.Host, cfg.Port))
	if err != nil {
		return err
	}
	s.listener = lis
	s.grpc = grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsCfg)), grpc.MaxRecvMsgSize(int(cfg.MaxArtifactBytes+1<<20)))
	nodev1.RegisterNodeServiceServer(s.grpc, s)
	go s.grpc.Serve(lis)
	return nil
}
func (s *Server) Stop() {
	if s.grpc != nil {
		s.grpc.GracefulStop()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

func (s *Server) RegisterNode(ctx context.Context, req *nodev1.RegisterNodeRequest) (*nodev1.RegisterNodeResponse, error) {
	if req.ProtocolVersion != ProtocolVersion {
		return nil, status.Error(codes.FailedPrecondition, "protocol version mismatch")
	}
	if req.Name == "" || req.Token == "" || len(req.CsrPem) == 0 {
		return nil, status.Error(codes.InvalidArgument, "name, token and CSR are required")
	}
	var created models.CertificateNode
	var certPEM []byte
	var expires time.Time
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var token models.NodeRegistrationToken
		if e := tx.Where("token_hash = ? AND used_at IS NULL AND revoked_at IS NULL AND expires_at > ?", db.HashToken(req.Token), time.Now()).First(&token).Error; e != nil {
			return e
		}
		csrBlock, _ := pem.Decode(req.CsrPem)
		if csrBlock == nil {
			return errors.New("invalid CSR")
		}
		csr, e := x509.ParseCertificateRequest(csrBlock.Bytes)
		if e != nil || csr.CheckSignature() != nil {
			return errors.New("invalid CSR signature")
		}
		created = models.CertificateNode{Name: req.Name, Remark: req.Remark, GroupID: token.GroupID, Status: "registered", Enabled: true, Capacity: max(int(req.Capacity), 1), ProtocolVersion: req.ProtocolVersion}
		if e = tx.Create(&created).Error; e != nil {
			return e
		}
		certPEM, expires, e = s.signNodeCertificate(created.ID, req.Name, csr)
		if e != nil {
			return e
		}
		block, _ := pem.Decode(certPEM)
		parsed, _ := x509.ParseCertificate(block.Bytes)
		created.IdentitySerial = parsed.SerialNumber.String()
		created.CertificateExpiresAt = &expires
		if e = tx.Save(&created).Error; e != nil {
			return e
		}
		now := time.Now()
		return tx.Model(&token).Update("used_at", now).Error
	})
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "registration token invalid or node name exists")
	}
	return &nodev1.RegisterNodeResponse{NodeId: uint64(created.ID), ClientCertificatePem: certPEM, ClientCaPem: s.caPEM, ExpiresUnix: expires.Unix()}, nil
}
func (s *Server) Connect(stream nodev1.NodeService_ConnectServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	if first.GetHello() == nil || first.NodeId == 0 {
		return status.Error(codes.InvalidArgument, "hello required")
	}
	if err = s.verifyPeerNode(stream.Context(), first.NodeId); err != nil {
		return err
	}
	nodeID := first.NodeId
	s.hub.Lock()
	s.hub.streams[nodeID] = &streamRef{stream: stream}
	s.hub.Unlock()
	defer func() {
		s.hub.Lock()
		delete(s.hub.streams, nodeID)
		s.hub.Unlock()
		db.DB.Model(&models.CertificateNode{}).Where("id = ?", nodeID).Updates(map[string]any{"status": "offline", "running": 0})
	}()
	hello := first.GetHello()
	now := time.Now()
	db.DB.Model(&models.CertificateNode{}).Where("id = ?", nodeID).Updates(map[string]any{"status": "online", "version": hello.Version, "capacity": max(int(hello.Capacity), 1), "providers": join(hello.Providers), "last_heartbeat_at": now})
	for {
		msg, e := stream.Recv()
		if e != nil {
			return e
		}
		if msg.NodeId != nodeID {
			continue
		}
		s.handleNodeMessage(msg)
	}
}
func (s *Server) handleNodeMessage(msg *nodev1.NodeMessage) {
	now := time.Now()
	if hb := msg.GetHeartbeat(); hb != nil {
		db.DB.Model(&models.CertificateNode{}).Where("id = ?", msg.NodeId).Updates(map[string]any{"status": "online", "running": hb.Running, "last_heartbeat_at": now})
		return
	}
	if stage := msg.GetStageEvent(); stage != nil {
		db.DB.Model(&models.NodeTaskLease{}).Where("lease_id = ? AND node_id = ?", msg.LeaseId, msg.NodeId).Updates(map[string]any{"stage": stage.Stage, "last_ack_at": now})
		db.DB.Model(&models.CertificateTask{}).Where("task_id = ? AND lease_id = ?", msg.TaskId, msg.LeaseId).Updates(map[string]any{"remote_stage": stage.Stage, "last_heartbeat_at": now})
		return
	}
	if ack := msg.GetAssignmentAck(); ack != nil {
		updates := map[string]any{"last_ack_at": now}
		if ack.Accepted {
			updates["stage"] = "assigned"
		} else {
			updates["stage"] = "rejected"
		}
		db.DB.Model(&models.NodeTaskLease{}).Where("lease_id = ? AND node_id = ?", msg.LeaseId, msg.NodeId).Updates(updates)
	}
}
func (s *Server) RotateIdentity(ctx context.Context, req *nodev1.RotateIdentityRequest) (*nodev1.RotateIdentityResponse, error) {
	if err := s.verifyPeerNode(ctx, req.NodeId); err != nil {
		return nil, err
	}
	block, _ := pem.Decode(req.CsrPem)
	if block == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid CSR")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, err
	}
	if err = csr.CheckSignature(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid CSR signature")
	}
	var node models.CertificateNode
	if err = db.DB.First(&node, req.NodeId).Error; err != nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	cert, expires, err := s.signNodeCertificate(uint(req.NodeId), node.Name, csr)
	if err != nil {
		return nil, err
	}
	block, _ = pem.Decode(cert)
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	if err = db.DB.Model(&node).Updates(map[string]any{"identity_serial": parsed.SerialNumber.String(), "certificate_expires_at": &expires}).Error; err != nil {
		return nil, err
	}
	return &nodev1.RotateIdentityResponse{ClientCertificatePem: cert, ExpiresUnix: expires.Unix()}, nil
}
func (s *Server) GetControllerInfo(context.Context, *nodev1.ControllerInfoRequest) (*nodev1.ControllerInfoResponse, error) {
	return &nodev1.ControllerInfoResponse{ProtocolVersion: ProtocolVersion, Version: "dev", HeartbeatSeconds: 10, MaxArtifactBytes: 10 << 20}, nil
}
func (s *Server) verifyPeerNode(ctx context.Context, nodeID uint64) error {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "mTLS required")
	}
	info, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(info.State.VerifiedChains) == 0 {
		return status.Error(codes.Unauthenticated, "mTLS required")
	}
	serial := info.State.PeerCertificates[0].SerialNumber.String()
	var n int64
	db.DB.Model(&models.CertificateNode{}).Where("id = ? AND identity_serial = ? AND enabled = ? AND revoked_at IS NULL", nodeID, serial, true).Count(&n)
	if n != 1 {
		return status.Error(codes.PermissionDenied, "node identity revoked")
	}
	return nil
}
func (s *Server) signNodeCertificate(id uint, name string, csr *x509.CertificateRequest) ([]byte, time.Time, error) {
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	expires := time.Now().Add(90 * 24 * time.Hour)
	tpl := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "domainsprite-node-" + strconv.FormatUint(uint64(id), 10), OrganizationalUnit: []string{name}}, NotBefore: time.Now().Add(-time.Minute), NotAfter: expires, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, tpl, s.caCert, csr.PublicKey, s.caKey)
	if err != nil {
		return nil, time.Time{}, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), expires, nil
}
func loadOrCreateCA() (*x509.Certificate, ed25519.PrivateKey, []byte, error) {
	var certSetting, keySetting models.SystemSetting
	certErr := db.DB.First(&certSetting, "key = ?", "node.ca_cert").Error
	keyErr := db.DB.First(&keySetting, "key = ?", "node.ca_key").Error
	if certErr == nil && keyErr == nil {
		plain, err := db.DecryptSecret(keySetting.Value)
		if err != nil {
			return nil, nil, nil, err
		}
		cb, _ := pem.Decode([]byte(certSetting.Value))
		kb, _ := pem.Decode([]byte(plain))
		if cb == nil || kb == nil {
			return nil, nil, nil, errors.New("节点 CA 数据损坏")
		}
		cert, _ := x509.ParseCertificate(cb.Bytes)
		keyAny, err := x509.ParsePKCS8PrivateKey(kb.Bytes)
		if err != nil {
			return nil, nil, nil, err
		}
		return cert, keyAny.(ed25519.PrivateKey), []byte(certSetting.Value), nil
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	tpl := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "DomainSprite Node CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(10 * 365 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, key.Public(), key)
	if err != nil {
		return nil, nil, nil, err
	}
	cert, _ := x509.ParseCertificate(der)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	pkcs, _ := x509.MarshalPKCS8PrivateKey(key)
	encrypted, err := db.EncryptSecret(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs})))
	if err != nil {
		return nil, nil, nil, err
	}
	if err = db.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&models.SystemSetting{Key: "node.ca_cert", Value: string(certPEM)}).Error; e != nil {
			return e
		}
		return tx.Create(&models.SystemSetting{Key: "node.ca_key", Value: encrypted, Sensitive: true}).Error
	}); err != nil {
		return nil, nil, nil, err
	}
	return cert, key, certPEM, nil
}
func join(v []string) string {
	out := ""
	for i, s := range v {
		if i > 0 {
			out += ","
		}
		out += s
	}
	return out
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
