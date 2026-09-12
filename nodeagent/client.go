package nodeagent

import (
	nodev1 "DDNSServer/api/node/v1"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const ProtocolVersion = "1"

type Config struct {
	NodeID         uint64 `json:"nodeId"`
	Name           string `json:"name"`
	Remark         string `json:"remark"`
	Controller     string `json:"controller"`
	CertificatePEM string `json:"certificatePem"`
	PrivateKeyPEM  string `json:"privateKeyPem"`
	ClientCAPEM    string `json:"clientCaPem"`
	StorageKey     string `json:"storageKey"`
	Capacity       int32  `json:"capacity"`
}

func Register(ctx context.Context, name, remark, controller, token string, capacity int32) (Config, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Config{}, err
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: name}}, key)
	if err != nil {
		return Config{}, err
	}
	target, serverName, err := grpcTarget(controller)
	if err != nil {
		return Config{}, err
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, ServerName: serverName})))
	if err != nil {
		return Config{}, err
	}
	defer conn.Close()
	resp, err := nodev1.NewNodeServiceClient(conn).RegisterNode(ctx, &nodev1.RegisterNodeRequest{ProtocolVersion: ProtocolVersion, Name: name, Remark: remark, Token: token, CsrPem: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}), Capacity: capacity})
	if err != nil {
		return Config{}, err
	}
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	storage := make([]byte, 32)
	_, _ = rand.Read(storage)
	return Config{NodeID: resp.NodeId, Name: name, Remark: remark, Controller: controller, CertificatePEM: string(resp.ClientCertificatePem), PrivateKeyPEM: string(keyPEM), ClientCAPEM: string(resp.ClientCaPem), StorageKey: fmt.Sprintf("%x", storage), Capacity: capacity}, nil
}
func SaveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	_ = f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}
func Run(ctx context.Context, cfg Config, version string) error {
	cert, err := tls.X509KeyPair([]byte(cfg.CertificatePEM), []byte(cfg.PrivateKeyPEM))
	if err != nil {
		return err
	}
	target, serverName, err := grpcTarget(cfg.Controller)
	if err != nil {
		return err
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, ServerName: serverName})))
	if err != nil {
		return err
	}
	defer conn.Close()
	stream, err := nodev1.NewNodeServiceClient(conn).Connect(ctx)
	if err != nil {
		return err
	}
	sender := &nodeSender{nodeID: cfg.NodeID, stream: stream}
	if err = sender.send(&nodev1.NodeMessage{Payload: &nodev1.NodeMessage_Hello{Hello: &nodev1.Hello{Version: version, Capacity: cfg.Capacity, Providers: []string{"Ali", "Tencent", "Cloudflare"}}}}); err != nil {
		return err
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	recvErr := make(chan error, 1)
	go func() {
		for {
			message, e := stream.Recv()
			if e != nil {
				recvErr <- e
				return
			}
			if message.GetAssignment() != nil {
				_ = sender.send(&nodev1.NodeMessage{TaskId: message.TaskId, LeaseId: message.LeaseId, Payload: &nodev1.NodeMessage_AssignmentAck{AssignmentAck: &nodev1.AssignmentAck{Accepted: false, Reason: "execution engine unavailable"}}})
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err = <-recvErr:
			return err
		case <-ticker.C:
			if err = sender.send(&nodev1.NodeMessage{Payload: &nodev1.NodeMessage_Heartbeat{Heartbeat: &nodev1.Heartbeat{}}}); err != nil {
				return err
			}
		}
	}
}

// nodeSender serializes client-stream writes. gRPC permits one reader and one
// writer concurrently, but not multiple concurrent writers.
type nodeSender struct {
	mu       sync.Mutex
	sequence uint64
	nodeID   uint64
	stream   nodev1.NodeService_ConnectClient
}

func (s *nodeSender) send(message *nodev1.NodeMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence++
	message.ProtocolVersion = ProtocolVersion
	message.NodeId = s.nodeID
	message.Sequence = s.sequence
	message.TimestampUnix = time.Now().Unix()
	return s.stream.Send(message)
}
func grpcTarget(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}
	if u.Scheme != "https" || u.Host == "" {
		return "", "", errors.New("controller must be an https URL")
	}
	return u.Host, u.Hostname(), nil
}
