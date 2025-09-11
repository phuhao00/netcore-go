// Package security 提供NetCore-Go网络库的安全功能
// Author: NetCore-Go Team
// Created: 2024

package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"sync"
	"time"
)

// TLSConfig TLS配置结构
type TLSConfig struct {
	// 证书文件路径
	CertFile string `json:"cert_file" yaml:"cert_file"`
	// 私钥文件路径
	KeyFile string `json:"key_file" yaml:"key_file"`
	// CA证书文件路径
	CAFile string `json:"ca_file" yaml:"ca_file"`
	// 是否启用客户端认证
	ClientAuth bool `json:"client_auth" yaml:"client_auth"`
	// 最小TLS版本
	MinVersion uint16 `json:"min_version" yaml:"min_version"`
	// 最大TLS版本
	MaxVersion uint16 `json:"max_version" yaml:"max_version"`
	// 支持的密码套件
	CipherSuites []uint16 `json:"cipher_suites" yaml:"cipher_suites"`
	// 是否启用OCSP装订
	OCSPStapling bool `json:"ocsp_stapling" yaml:"ocsp_stapling"`
	// 服务器名称
	ServerName string `json:"server_name" yaml:"server_name"`
	// 是否跳过证书验证（仅用于测试）
	InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	// 证书自动更新间隔
	AutoRenewInterval time.Duration `json:"auto_renew_interval" yaml:"auto_renew_interval"`
}

// TLSManager TLS管理器
type TLSManager struct {
	config     *TLSConfig
	tlsConfig  *tls.Config
	certificate *tls.Certificate
	mu         sync.RWMutex
	autoRenew  bool
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// DefaultTLSConfig 返回默认TLS配置
func DefaultTLSConfig() *TLSConfig {
	return &TLSConfig{
		MinVersion:        tls.VersionTLS12,
		MaxVersion:        tls.VersionTLS13,
		ClientAuth:        false,
		OCSPStapling:      true,
		InsecureSkipVerify: false,
		AutoRenewInterval: 24 * time.Hour,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}
}

// NewTLSManager 创建TLS管理器
func NewTLSManager(config *TLSConfig) (*TLSManager, error) {
	if config == nil {
		config = DefaultTLSConfig()
	}

	manager := &TLSManager{
		config:   config,
		stopChan: make(chan struct{}),
	}

	// 加载TLS配置
	if err := manager.loadTLSConfig(); err != nil {
		return nil, fmt.Errorf("failed to load TLS config: %w", err)
	}

	return manager, nil
}

// loadTLSConfig 加载TLS配置
func (tm *TLSManager) loadTLSConfig() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tlsConfig := &tls.Config{
		MinVersion:         tm.config.MinVersion,
		MaxVersion:         tm.config.MaxVersion,
		CipherSuites:       tm.config.CipherSuites,
		InsecureSkipVerify: tm.config.InsecureSkipVerify,
		ServerName:         tm.config.ServerName,
	}

	// 设置客户端认证模式
	if tm.config.ClientAuth {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	} else {
		tlsConfig.ClientAuth = tls.NoClientCert
	}

	// 加载证书
	if tm.config.CertFile != "" && tm.config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tm.config.CertFile, tm.config.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load certificate: %w", err)
		}
		tm.certificate = &cert
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// 加载CA证书
	if tm.config.CAFile != "" {
		caCert, err := ioutil.ReadFile(tm.config.CAFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse CA certificate")
		}

		if tm.config.ClientAuth {
			tlsConfig.ClientCAs = caCertPool
		} else {
			tlsConfig.RootCAs = caCertPool
		}
	}

	tm.tlsConfig = tlsConfig
	return nil
}

// GetTLSConfig 获取TLS配置
func (tm *TLSManager) GetTLSConfig() *tls.Config {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tlsConfig.Clone()
}

// StartAutoRenew 启动证书自动更新
func (tm *TLSManager) StartAutoRenew() {
	if tm.config.AutoRenewInterval <= 0 {
		return
	}

	tm.autoRenew = true
	tm.wg.Add(1)
	go tm.autoRenewLoop()
}

// StopAutoRenew 停止证书自动更新
func (tm *TLSManager) StopAutoRenew() {
	if !tm.autoRenew {
		return
	}

	close(tm.stopChan)
	tm.wg.Wait()
	tm.autoRenew = false
}

// autoRenewLoop 证书自动更新循环
func (tm *TLSManager) autoRenewLoop() {
	defer tm.wg.Done()

	ticker := time.NewTicker(tm.config.AutoRenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tm.stopChan:
			return
		case <-ticker.C:
			if err := tm.checkAndRenewCertificate(); err != nil {
				// 记录错误，但不停止自动更新
				fmt.Printf("Certificate renewal failed: %v\n", err)
			}
		}
	}
}

// checkAndRenewCertificate 检查并更新证书
func (tm *TLSManager) checkAndRenewCertificate() error {
	tm.mu.RLock()
	cert := tm.certificate
	tm.mu.RUnlock()

	if cert == nil {
		return nil
	}

	// 解析证书
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// 检查证书是否即将过期（30天内）
	if time.Until(x509Cert.NotAfter) > 30*24*time.Hour {
		return nil // 证书还有效
	}

	// 重新加载证书
	return tm.loadTLSConfig()
}

// GenerateSelfSignedCert 生成自签名证书
func (tm *TLSManager) GenerateSelfSignedCert(hosts []string, certFile, keyFile string) error {
	// 生成私钥
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"NetCore-Go"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour), // 1年有效期
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// 添加主机名
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}

	// 生成证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// 保存证书
	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// 保存私钥
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privDER}); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	return nil
}

// ValidateCertificate 验证证书
func (tm *TLSManager) ValidateCertificate() error {
	tm.mu.RLock()
	cert := tm.certificate
	tm.mu.RUnlock()

	if cert == nil {
		return fmt.Errorf("no certificate loaded")
	}

	// 解析证书
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// 检查证书有效期
	now := time.Now()
	if now.Before(x509Cert.NotBefore) {
		return fmt.Errorf("certificate is not yet valid")
	}
	if now.After(x509Cert.NotAfter) {
		return fmt.Errorf("certificate has expired")
	}

	// 验证证书链
	if tm.config.CAFile != "" {
		caCert, err := ioutil.ReadFile(tm.config.CAFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse CA certificate")
		}

		opts := x509.VerifyOptions{
			Roots: caCertPool,
		}

		if _, err := x509Cert.Verify(opts); err != nil {
			return fmt.Errorf("certificate verification failed: %w", err)
		}
	}

	return nil
}

// GetCertificateInfo 获取证书信息
func (tm *TLSManager) GetCertificateInfo() (*CertificateInfo, error) {
	tm.mu.RLock()
	cert := tm.certificate
	tm.mu.RUnlock()

	if cert == nil {
		return nil, fmt.Errorf("no certificate loaded")
	}

	// 解析证书
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &CertificateInfo{
		Subject:    x509Cert.Subject.String(),
		Issuer:     x509Cert.Issuer.String(),
		NotBefore:  x509Cert.NotBefore,
		NotAfter:   x509Cert.NotAfter,
		DNSNames:   x509Cert.DNSNames,
		IPAddresses: x509Cert.IPAddresses,
		SerialNumber: x509Cert.SerialNumber.String(),
	}, nil
}

// CertificateInfo 证书信息
type CertificateInfo struct {
	Subject      string        `json:"subject"`
	Issuer       string        `json:"issuer"`
	NotBefore    time.Time     `json:"not_before"`
	NotAfter     time.Time     `json:"not_after"`
	DNSNames     []string      `json:"dns_names"`
	IPAddresses  []net.IP      `json:"ip_addresses"`
	SerialNumber string        `json:"serial_number"`
}

// IsExpiringSoon 检查证书是否即将过期
func (ci *CertificateInfo) IsExpiringSoon(threshold time.Duration) bool {
	return time.Until(ci.NotAfter) <= threshold
}

// IsValid 检查证书是否有效
func (ci *CertificateInfo) IsValid() bool {
	now := time.Now()
	return now.After(ci.NotBefore) && now.Before(ci.NotAfter)
}