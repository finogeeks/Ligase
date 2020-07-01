// Copyright (C) 2020 Finogeeks Co., Ltd
//
// This program is free software: you can redistribute it and/or  modify
// it under the terms of the GNU Affero General Public License, version 3,
// as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cert

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/finogeeks/ligase/common"
	"github.com/finogeeks/ligase/common/fetch"
	"github.com/finogeeks/ligase/federation/fedutil"
	"github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

type Cert struct {
	Certs           sync.Map
	ticker          *time.Timer
	notaryEnable    bool
	notaryRootCAUrl string
	notaryCertUrl   string
	notaryCRLUrl    string
	serverName      []string
	keyDB           model.KeyDatabase
}

var (
	//Certs sync.Map
	//ticker *time.Timer
	// CRLSet  = map[string]*pkix.CertificateList{}
	// crlLock = new(sync.Mutex)
	certIns *Cert
	once    sync.Once
)

type CRLResponse struct {
	CrlBase64   string              `json:"crl"`
	CrlSnapshot map[string][]string `json:"crl_snapshot"`
}

func NewCert(
	notaryEnable bool,
	notaryRootCAUrl, notaryCertUrl, notaryCRLUrl string,
	serverName []string,
	keyDB model.KeyDatabase,
) *Cert {
	once.Do(func() {
		certIns = &Cert{
			notaryEnable:    notaryEnable,
			notaryRootCAUrl: notaryRootCAUrl,
			notaryCertUrl:   notaryCertUrl,
			notaryCRLUrl:    notaryCRLUrl,
			serverName:      serverName,
			keyDB:           keyDB,
			ticker:          time.NewTimer(time.Second * 5),
		}
	})
	return certIns
}

func (c *Cert) GetCerts() *sync.Map {
	return &c.Certs
}

func (c *Cert) Load() error {
	c.Certs.Store("httpsCliEnable", c.notaryEnable)
	if c.notaryEnable == false {
		log.Infof("--------------notary service is disable")
		return nil
	}

	rootCA, serverCert, serverKey, crl, _ := c.keyDB.SelectAllCerts(context.TODO())
	if rootCA == "" {
		resp, err := fedutil.DownloadFromNotary("rootCA", c.notaryRootCAUrl, c.keyDB)
		if err != nil {
			return err
		}
		rootCA = resp.RootCA
	}

	// update CRL whenever reboot
	resp, err := fedutil.DownloadFromNotary("crl", c.notaryCRLUrl, c.keyDB)
	if err == nil && resp.CRL != "" {
		crl = resp.CRL
	}

	// self check
	go c.startCheckTimer()
	ok := false
	if serverCert != "" {
		ok, _ = c.CheckCert(crl, serverCert)
	}

	// update then check again
	if serverCert == "" || serverKey == "" || !ok {
		reqUrl := fmt.Sprintf(c.notaryCertUrl, c.serverName[0])
		resp, err := fedutil.DownloadFromNotary("cert", reqUrl, c.keyDB)
		if err != nil {
			return err
		}
		serverCert = resp.ServerCert
		serverKey = resp.ServerKey

		ok, err = c.CheckCert(crl, serverCert)
		if !ok {
			return err
		}
	}

	c.Certs.Store("rootCA", rootCA)
	c.Certs.Store("crl", crl)
	c.Certs.Store("serverCert", serverCert)
	c.Certs.Store("serverKey", serverKey)

	log.Infof("load certs succeed")

	// TODO: check fed domains' cert
	// fetchDomainsCert()
	// CheckCert()

	return nil
}

// TODO: check other domains' cert regularly
func (c *Cert) fetchDomainsCert() {

}

func (c *Cert) CheckCert(crl, certPem string) (bool, error) {
	cert, err := c.ToCert([]byte(certPem))
	if err != nil {
		c.Certs.Store("revoked", true)
		return false, err
	}
	revoked, ok, err := c.verifyCert(crl, cert)
	if revoked || !ok {
		c.Certs.Store("revoked", true)
		return false, err
	}
	c.Certs.Store("revoked", false)

	// reset checking cert timer
	c.setCertTimer(cert)

	return true, err
}

func (c *Cert) verifyCert(crlBase64 string, cert *x509.Certificate) (revoked, ok bool, err error) {
	if cert == nil {
		return false, false, errors.New("cert is nil")
	}
	if !time.Now().Before(cert.NotAfter) {
		msg := fmt.Sprintf("certificate expired %s\n", cert.NotAfter)
		return true, true, errors.New(msg)
	} else if !time.Now().After(cert.NotBefore) {
		msg := fmt.Sprintf("certificate isn't valid until %s\n", cert.NotBefore)
		return true, true, errors.New(msg)
	}

	// revoked check
	if crlBase64 == "" {
		return false, false, nil
	}
	b := common.FromBase64(crlBase64)
	crl, _ := x509.ParseCRL(b)
	for _, revoked := range crl.TBSCertList.RevokedCertificates {
		if cert.SerialNumber.Cmp(revoked.SerialNumber) == 0 {
			msg := "serial number match: intermediate is revoked\n"
			return true, true, errors.New(msg)
		}
	}

	return false, true, nil
}

func (c *Cert) setCertTimer(cert *x509.Certificate) {
	if cert != nil {
		timeout := time.Second
		now := time.Now()
		if now.Before(cert.NotAfter) {
			validTime := cert.NotAfter.Sub(now)
			if validTime <= time.Hour {
				timeout = validTime + time.Second*2
			} else if validTime >= time.Hour*24 {
				timeout = time.Hour * 24
			} else {
				timeout = time.Hour
			}
		}
		c.ticker.Reset(timeout)
		log.Infof("----------------------------- cert reset timeout: %v", timeout)
	}
}

func (c *Cert) startCheckTimer() {
	for {
		select {
		case <-c.ticker.C:
			log.Infof("----------------------------- timeout,  start to check cert")
			crl, _ := c.Certs.Load("crl")
			serverCert, _ := c.Certs.Load("serverCert")
			if ok, err := c.CheckCert(crl.(string), serverCert.(string)); !ok {
				log.Warnf(err.Error())
			}
		}
	}
}

func (c *Cert) ToCert(pemCerts []byte) (*x509.Certificate, error) {
	if pemCerts == nil {
		return nil, errors.New("pemCerts is nil")
	}

	// for len(pemCerts) > 0 {
	var block *pem.Block
	block, pemCerts = pem.Decode(pemCerts)
	if block == nil {
		return nil, errors.New("keyBlock is nil")
	}
	if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
		return nil, errors.New("keyBlock type is not CERTIFICATE")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	// }
	return cert, nil
}

func (c *Cert) fetchCRL(url string) (*pkix.CertificateList, error) {
	resp, err := fetch.HttpsReqUnsafe("GET", url, nil, nil)
	if err != nil {
		return nil, err
	} else if resp.StatusCode >= 300 {
		return nil, errors.New("failed to retrieve CRL")
	}

	var w CRLResponse
	if err := json.Unmarshal(resp.Body, &w); err != nil {
		return nil, errors.New("failed to unmarshal CRL")
	}
	b := common.FromBase64(w.CrlBase64)
	return x509.ParseCRL(b)
}
