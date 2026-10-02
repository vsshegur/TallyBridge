package app

import (
	"errors"
	"strings"
	"time"
)

func companyProfileMap(p CompanyProfile) map[string]string {
	return map[string]string{
		"name": p.Name, "gstin": p.GSTIN, "address": p.Address, "city": p.City,
		"state": p.State, "country": p.Country, "pincode": p.Pincode,
		"phone": p.Phone, "email": p.Email, "stateCode": p.StateCode,
	}
}
func profileFromDetails(company string, d map[string]string) CompanyProfile {
	p := CompanyProfile{
		Name:    firstNonBlank(strings.TrimSpace(d["name"]), strings.TrimSpace(company)),
		GSTIN:   strings.ToUpper(strings.TrimSpace(d["gstin"])),
		Address: strings.TrimSpace(d["address"]), City: strings.TrimSpace(d["city"]),
		State: strings.TrimSpace(d["state"]), Country: strings.TrimSpace(d["country"]),
		Pincode: strings.TrimSpace(d["pincode"]), Phone: strings.TrimSpace(d["phone"]),
		Email: strings.TrimSpace(d["email"]), StateCode: strings.TrimSpace(d["stateCode"]),
	}
	if p.StateCode == "" && len(p.GSTIN) >= 2 {
		p.StateCode = p.GSTIN[:2]
	}
	return p
}
func (s *Server) savedCompanyProfile(company string) CompanyProfile {
	company = strings.TrimSpace(company)
	var p CompanyProfile
	if _, ok := s.cache.Get("company-profile-v1", company, "", &p); ok {
		if p.Name == "" {
			p.Name = company
		}
		return p
	}
	var legacy map[string]string
	if _, ok := s.cache.Get("proforma-company", company, "", &legacy); ok {
		p = profileFromDetails(company, legacy)
	}
	if p.Name == "" {
		p.Name = company
	}
	return p
}
func (s *Server) companyProfile(company string, refresh bool) (CompanyProfile, error) {
	company = strings.TrimSpace(company)
	if company == "" {
		return CompanyProfile{}, errors.New("company required")
	}
	var cached CompanyProfile
	at, have := s.cache.Get("company-profile-v1", company, "", &cached)
	if have && !refresh && time.Since(at) < 12*time.Hour && cached.Name != "" && (cached.GSTIN != "" || cached.Address != "") {
		return cached, nil
	}
	details := map[string]string{}
	if have {
		mergeSupplierDetails(details, companyProfileMap(cached))
	}
	var legacy map[string]string
	if _, ok := s.cache.Get("proforma-company", company, "", &legacy); ok {
		mergeSupplierDetails(details, legacy)
	}
	var lastErr error
	if raw, _, _, err := s.tally.request("Read company identity", company, companyDetailsXML(company), 0); err == nil {
		mergeSupplierDetails(details, supplierDetailsFromXML(raw))
	} else {
		lastErr = err
	}
	if raw, _, _, err := s.tally.request("Read company tax unit", company, taxUnitsXML(company), 0); err == nil {
		mergeSupplierDetails(details, supplierDetailsFromXML(raw))
	} else if lastErr == nil {
		lastErr = err
	}
	if details["gstin"] == "" {
		if raw, _, _, err := s.tally.request("Read company GSTIN", company, companyGSTXML(company), 0); err == nil {
			mergeSupplierDetails(details, supplierDetailsFromXML(raw))
		} else if lastErr == nil {
			lastErr = err
		}
	}
	p := profileFromDetails(company, details)
	if p.Name == "" {
		p.Name = company
	}
	if p.GSTIN != "" || p.Address != "" || p.Phone != "" || p.Email != "" {
		_ = s.cache.Put("company-profile-v1", company, "", p, time.Now())
		_ = s.cache.Put("proforma-company", company, "", companyProfileMap(p), time.Now())
		return p, nil
	}
	if have {
		return cached, lastErr
	}
	return p, lastErr
}

func companyProfileAddress(p CompanyProfile) string {
	parts := []string{}
	seen := map[string]bool{}
	add := func(v string) {
		v = strings.Trim(strings.TrimSpace(v), ",")
		if v == "" {
			return
		}
		k := strings.ToLower(v)
		if !seen[k] {
			seen[k] = true
			parts = append(parts, v)
		}
	}
	add(p.Address)
	add(p.City)
	add(p.State)
	add(p.Pincode)
	add(p.Country)
	return strings.Join(parts, ", ")
}
