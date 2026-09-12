package certificate

import (
	"DDNSServer/models"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"sort"
	"strings"

	"golang.org/x/net/idna"
)

const MaxBaseDomains = 50

func NormalizeDomain(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
	if raw == "" || strings.Contains(raw, "://") || strings.ContainsAny(raw, "/*:@ ") || net.ParseIP(raw) != nil {
		return "", errors.New("域名格式无效")
	}
	if u, err := url.Parse("//" + raw); err != nil || u.Hostname() != raw {
		return "", errors.New("域名不能包含端口或路径")
	}
	name, err := idna.Lookup.ToASCII(raw)
	if err != nil || len(name) > 253 {
		return "", errors.New("域名格式无效")
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return "", errors.New("域名必须包含有效后缀")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("域名标签无效")
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return "", errors.New("域名包含非法字符")
			}
		}
	}
	return name, nil
}

func NormalizeCertificateDomains(in []models.CertificateDomain) ([]models.CertificateDomain, []string, string, error) {
	seen := map[string]bool{}
	out := make([]models.CertificateDomain, 0, len(in))
	for _, d := range in {
		name, err := NormalizeDomain(d.DomainName)
		if err != nil {
			return nil, nil, "", err
		}
		if !seen[name] {
			d.DomainName = name
			if d.ChallengeMode == "" {
				d.ChallengeMode = "direct"
			}
			if d.ChallengeMode != "direct" && d.ChallengeMode != "delegated" {
				return nil, nil, "", errors.New("challengeMode 无效")
			}
			seen[name] = true
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return nil, nil, "", errors.New("域名列表不能为空")
	}
	if len(out) > MaxBaseDomains {
		return nil, nil, "", errors.New("单张证书最多包含 50 个基础域名")
	}
	sans := make([]string, 0, len(out)*2)
	for _, d := range out {
		sans = append(sans, d.DomainName, "*."+d.DomainName)
	}
	sort.Strings(sans)
	sum := sha256.Sum256([]byte(strings.Join(sans, "\n")))
	return out, sans, hex.EncodeToString(sum[:]), nil
}

func DelegatedRecordName(domain string, prefix ...string) string {
	sum := sha256.Sum256([]byte(domain))
	name := hex.EncodeToString(sum[:16])
	if len(prefix) > 0 && strings.Trim(prefix[0], ". ") != "" {
		name = strings.Trim(prefix[0], ". ") + "." + name
	}
	return name
}

func NormalizeDelegationPrefix(raw string) (string, error) {
	raw = strings.ToLower(strings.Trim(raw, ". "))
	if raw == "" {
		return "", nil
	}
	normalized, err := NormalizeDomain(raw + ".example")
	if err != nil {
		return "", errors.New("承载前缀必须由合法 DNS 标签组成")
	}
	return strings.TrimSuffix(normalized, ".example"), nil
}
