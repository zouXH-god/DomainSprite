package models

import "testing"

type fakeDNSProvider struct {
	account Account
	added   []RecordInfo
	deleted []RecordInfo
	next    int
}

func (f *fakeDNSProvider) GetAccountInfo() Account                         { return f.account }
func (f *fakeDNSProvider) GetDomainList(DomainsSearch) (DomainList, error) { return DomainList{}, nil }
func (f *fakeDNSProvider) GetRecordList(DNSSearch) (RecordInfoList, error) {
	return RecordInfoList{}, nil
}
func (f *fakeDNSProvider) AddRecord(r RecordInfo) (RecordInfo, error) {
	f.next++
	r.Id = string(rune('0' + f.next))
	f.added = append(f.added, r)
	return r, nil
}
func (f *fakeDNSProvider) UpdateRecord(r RecordInfo) (RecordInfo, error) { return r, nil }
func (f *fakeDNSProvider) DeleteRecord(domain, id string) (RecordInfo, error) {
	f.deleted = append(f.deleted, RecordInfo{DomainName: domain, Id: id})
	return RecordInfo{}, nil
}
func (f *fakeDNSProvider) SetRecordStatus(string, string, string) (RecordInfo, error) {
	return RecordInfo{}, nil
}
func (f *fakeDNSProvider) GetRecordInfo(string, string) (RecordInfo, error) { return RecordInfo{}, nil }

func TestMultiProviderRoutesChallengesByDomain(t *testing.T) {
	a := &fakeDNSProvider{account: Account{Name: "a"}}
	b := &fakeDNSProvider{account: Account{Name: "b"}}
	p := NewMultiProvider(map[string]ChallengeTarget{"one.example": {Provider: a, Domain: DomainInfo{Domains: Domains{Id: "zone-a", DomainName: "one.example"}}, Mode: "direct"}, "two.example": {Provider: b, Domain: DomainInfo{Domains: Domains{Id: "zone-b", DomainName: "two.example"}}, Mode: "direct"}}, t.TempDir())
	if err := p.Present("one.example", "", "auth-one"); err != nil {
		t.Fatal(err)
	}
	if err := p.Present("two.example", "", "auth-two"); err != nil {
		t.Fatal(err)
	}
	if len(a.added) != 1 || a.added[0].DomainId != "zone-a" || len(b.added) != 1 || b.added[0].DomainId != "zone-b" {
		t.Fatalf("challenge routed incorrectly: %#v %#v", a.added, b.added)
	}
	if err := p.CleanUp("one.example", "", "auth-one"); err != nil {
		t.Fatal(err)
	}
	if len(a.deleted) != 1 || len(b.deleted) != 0 {
		t.Fatal("cleanup used wrong provider")
	}
}

func TestCleanupPendingRemovesEveryPresentedRecordByID(t *testing.T) {
	provider := &fakeDNSProvider{account: Account{Name: "a"}}
	p := NewMultiProvider(map[string]ChallengeTarget{
		"one.example": {Provider: provider, Domain: DomainInfo{Domains: Domains{Id: "zone-a", DomainName: "one.example"}}, Mode: "direct"},
	}, t.TempDir())
	if err := p.Present("one.example", "", "auth-one"); err != nil {
		t.Fatal(err)
	}
	if err := p.Present("*.one.example", "", "auth-wildcard"); err != nil {
		t.Fatal(err)
	}
	p.CleanupPending()
	if len(provider.deleted) != 2 || provider.deleted[0].Id == "" || provider.deleted[1].Id == "" {
		t.Fatalf("pending challenges were not deleted exactly: %#v", provider.deleted)
	}
	p.CleanupPending()
	if len(provider.deleted) != 2 {
		t.Fatal("cleanup must be idempotent")
	}
}
