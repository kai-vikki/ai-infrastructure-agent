package agent

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/versus-control/ai-infrastructure-agent/internal/logging"
)

// =============================================================================
// COMMUNICATION SECURITY
// =============================================================================

// CommunicationSecurityManager manages security for agent communication
type CommunicationSecurityManager struct {
	encryptionKeys  map[string]*EncryptionKey
	signingKeys     map[string]*SigningKey
	trustedAgents   map[string]*TrustedAgent
	securityPolicies map[string]*SecurityPolicy
	logger          *logging.Logger
	mu              sync.RWMutex
}

// EncryptionKey represents an encryption key
type EncryptionKey struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	KeyType     KeyType                `json:"key_type"`
	PublicKey   []byte                 `json:"public_key"`
	PrivateKey  []byte                 `json:"private_key"`
	CreatedAt   time.Time              `json:"created_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Active      bool                   `json:"active"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SigningKey represents a signing key
type SigningKey struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	KeyType     KeyType                `json:"key_type"`
	PublicKey   []byte                 `json:"public_key"`
	PrivateKey  []byte                 `json:"private_key"`
	CreatedAt   time.Time              `json:"created_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Active      bool                   `json:"active"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// TrustedAgent represents a trusted agent
type TrustedAgent struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	AgentType   AgentType              `json:"agent_type"`
	PublicKey   []byte                 `json:"public_key"`
	TrustLevel  TrustLevel             `json:"trust_level"`
	CreatedAt   time.Time              `json:"created_at"`
	LastSeen    time.Time              `json:"last_seen"`
	Active      bool                   `json:"active"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SecurityPolicy represents a security policy
type SecurityPolicy struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Rules       []*SecurityRule        `json:"rules"`
	Active      bool                   `json:"active"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SecurityRule represents a security rule
type SecurityRule struct {
	ID          string                 `json:"id"`
	Type        SecurityRuleType       `json:"type"`
	Condition   string                 `json:"condition"`
	Action      SecurityAction         `json:"action"`
	Priority    int                    `json:"priority"`
	Active      bool                   `json:"active"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SecureMessage represents a secure message
type SecureMessage struct {
	ID            string                 `json:"id"`
	OriginalMessage *Message             `json:"original_message"`
	EncryptedData []byte                 `json:"encrypted_data"`
	Signature     []byte                 `json:"signature"`
	EncryptionKey string                 `json:"encryption_key"`
	SigningKey    string                 `json:"signing_key"`
	Timestamp     time.Time              `json:"timestamp"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// KeyType represents the type of key
type KeyType string

const (
	KeyTypeRSA KeyType = "rsa"
	KeyTypeAES KeyType = "aes"
)

// TrustLevel represents the trust level of an agent
type TrustLevel string

const (
	TrustLevelHigh   TrustLevel = "high"
	TrustLevelMedium TrustLevel = "medium"
	TrustLevelLow    TrustLevel = "low"
	TrustLevelNone   TrustLevel = "none"
)

// SecurityRuleType represents the type of security rule
type SecurityRuleType string

const (
	SecurityRuleTypeEncryption SecurityRuleType = "encryption"
	SecurityRuleTypeSigning    SecurityRuleType = "signing"
	SecurityRuleTypeAccess     SecurityRuleType = "access"
	SecurityRuleTypeRateLimit  SecurityRuleType = "rate_limit"
)

// SecurityAction represents a security action
type SecurityAction string

const (
	SecurityActionAllow    SecurityAction = "allow"
	SecurityActionDeny     SecurityAction = "deny"
	SecurityActionEncrypt  SecurityAction = "encrypt"
	SecurityActionSign     SecurityAction = "sign"
	SecurityActionLog      SecurityAction = "log"
	SecurityActionAlert    SecurityAction = "alert"
)

// NewCommunicationSecurityManager creates a new communication security manager
func NewCommunicationSecurityManager(logger *logging.Logger) *CommunicationSecurityManager {
	return &CommunicationSecurityManager{
		encryptionKeys:   make(map[string]*EncryptionKey),
		signingKeys:      make(map[string]*SigningKey),
		trustedAgents:    make(map[string]*TrustedAgent),
		securityPolicies: make(map[string]*SecurityPolicy),
		logger:           logger,
	}
}

// =============================================================================
// KEY MANAGEMENT
// =============================================================================

// GenerateEncryptionKey generates a new encryption key
func (csm *CommunicationSecurityManager) GenerateEncryptionKey(agentID string, keyType KeyType) (*EncryptionKey, error) {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	var publicKey, privateKey []byte
	var err error

	switch keyType {
	case KeyTypeRSA:
		publicKey, privateKey, err = csm.generateRSAKeyPair()
	case KeyTypeAES:
		publicKey, privateKey, err = csm.generateAESKey()
	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	key := &EncryptionKey{
		ID:         uuid.New().String(),
		AgentID:    agentID,
		KeyType:    keyType,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		CreatedAt:  time.Now(),
		Active:     true,
		Metadata:   make(map[string]interface{}),
	}

	csm.encryptionKeys[key.ID] = key

	csm.logger.WithFields(map[string]interface{}{
		"key_id":   key.ID,
		"agent_id": agentID,
		"key_type": keyType,
	}).Info("Encryption key generated successfully")

	return key, nil
}

// GenerateSigningKey generates a new signing key
func (csm *CommunicationSecurityManager) GenerateSigningKey(agentID string, keyType KeyType) (*SigningKey, error) {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	var publicKey, privateKey []byte
	var err error

	switch keyType {
	case KeyTypeRSA:
		publicKey, privateKey, err = csm.generateRSAKeyPair()
	case KeyTypeAES:
		return nil, fmt.Errorf("AES keys cannot be used for signing")
	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	key := &SigningKey{
		ID:         uuid.New().String(),
		AgentID:    agentID,
		KeyType:    keyType,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		CreatedAt:  time.Now(),
		Active:     true,
		Metadata:   make(map[string]interface{}),
	}

	csm.signingKeys[key.ID] = key

	csm.logger.WithFields(map[string]interface{}{
		"key_id":   key.ID,
		"agent_id": agentID,
		"key_type": keyType,
	}).Info("Signing key generated successfully")

	return key, nil
}

// generateRSAKeyPair generates an RSA key pair
func (csm *CommunicationSecurityManager) generateRSAKeyPair() ([]byte, []byte, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA private key: %w", err)
	}

	// Encode private key
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Encode public key
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&privateKey.PublicKey),
	})

	return publicKeyPEM, privateKeyPEM, nil
}

// generateAESKey generates an AES key
func (csm *CommunicationSecurityManager) generateAESKey() ([]byte, []byte, error) {
	// Generate 256-bit key
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, nil, fmt.Errorf("failed to generate AES key: %w", err)
	}

	// For AES, we use the same key for both public and private
	// In a real system, you might want to derive different keys
	return key, key, nil
}

// =============================================================================
// MESSAGE ENCRYPTION AND DECRYPTION
// =============================================================================

// EncryptMessage encrypts a message
func (csm *CommunicationSecurityManager) EncryptMessage(ctx context.Context, message *Message, encryptionKeyID string) (*SecureMessage, error) {
	csm.mu.RLock()
	encryptionKey, exists := csm.encryptionKeys[encryptionKeyID]
	csm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("encryption key %s not found", encryptionKeyID)
	}

	if !encryptionKey.Active {
		return nil, fmt.Errorf("encryption key %s is not active", encryptionKeyID)
	}

	// Serialize message
	messageData, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize message: %w", err)
	}

	// Encrypt message
	var encryptedData []byte
	switch encryptionKey.KeyType {
	case KeyTypeRSA:
		encryptedData, err = csm.encryptWithRSA(messageData, encryptionKey.PublicKey)
	case KeyTypeAES:
		encryptedData, err = csm.encryptWithAES(messageData, encryptionKey.PublicKey)
	default:
		return nil, fmt.Errorf("unsupported encryption key type: %s", encryptionKey.KeyType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message: %w", err)
	}

	secureMessage := &SecureMessage{
		ID:              uuid.New().String(),
		OriginalMessage: message,
		EncryptedData:   encryptedData,
		EncryptionKey:   encryptionKeyID,
		Timestamp:       time.Now(),
		Metadata:        make(map[string]interface{}),
	}

	csm.logger.WithFields(map[string]interface{}{
		"secure_message_id": secureMessage.ID,
		"original_message_id": message.ID,
		"encryption_key_id": encryptionKeyID,
	}).Debug("Message encrypted successfully")

	return secureMessage, nil
}

// DecryptMessage decrypts a secure message
func (csm *CommunicationSecurityManager) DecryptMessage(ctx context.Context, secureMessage *SecureMessage) (*Message, error) {
	csm.mu.RLock()
	encryptionKey, exists := csm.encryptionKeys[secureMessage.EncryptionKey]
	csm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("encryption key %s not found", secureMessage.EncryptionKey)
	}

	if !encryptionKey.Active {
		return nil, fmt.Errorf("encryption key %s is not active", secureMessage.EncryptionKey)
	}

	// Decrypt message
	var decryptedData []byte
	var err error

	switch encryptionKey.KeyType {
	case KeyTypeRSA:
		decryptedData, err = csm.decryptWithRSA(secureMessage.EncryptedData, encryptionKey.PrivateKey)
	case KeyTypeAES:
		decryptedData, err = csm.decryptWithAES(secureMessage.EncryptedData, encryptionKey.PrivateKey)
	default:
		return nil, fmt.Errorf("unsupported encryption key type: %s", encryptionKey.KeyType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to decrypt message: %w", err)
	}

	// Deserialize message
	var message Message
	if err := json.Unmarshal(decryptedData, &message); err != nil {
		return nil, fmt.Errorf("failed to deserialize message: %w", err)
	}

	csm.logger.WithFields(map[string]interface{}{
		"secure_message_id": secureMessage.ID,
		"decrypted_message_id": message.ID,
		"encryption_key_id": secureMessage.EncryptionKey,
	}).Debug("Message decrypted successfully")

	return &message, nil
}

// encryptWithRSA encrypts data with RSA
func (csm *CommunicationSecurityManager) encryptWithRSA(data []byte, publicKeyPEM []byte) ([]byte, error) {
	// Parse public key
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	// Encrypt data
	encryptedData, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}

	return encryptedData, nil
}

// decryptWithRSA decrypts data with RSA
func (csm *CommunicationSecurityManager) decryptWithRSA(encryptedData []byte, privateKeyPEM []byte) ([]byte, error) {
	// Parse private key
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Decrypt data
	decryptedData, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return decryptedData, nil
}

// encryptWithAES encrypts data with AES
func (csm *CommunicationSecurityManager) encryptWithAES(data []byte, key []byte) ([]byte, error) {
	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	encryptedData := gcm.Seal(nonce, nonce, data, nil)

	return encryptedData, nil
}

// decryptWithAES decrypts data with AES
func (csm *CommunicationSecurityManager) decryptWithAES(encryptedData []byte, key []byte) ([]byte, error) {
	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("encrypted data too short")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]

	// Decrypt data
	decryptedData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return decryptedData, nil
}

// =============================================================================
// MESSAGE SIGNING AND VERIFICATION
// =============================================================================

// SignMessage signs a message
func (csm *CommunicationSecurityManager) SignMessage(ctx context.Context, message *Message, signingKeyID string) (*SecureMessage, error) {
	csm.mu.RLock()
	signingKey, exists := csm.signingKeys[signingKeyID]
	csm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("signing key %s not found", signingKeyID)
	}

	if !signingKey.Active {
		return nil, fmt.Errorf("signing key %s is not active", signingKeyID)
	}

	// Serialize message
	messageData, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize message: %w", err)
	}

	// Sign message
	signature, err := csm.signWithRSA(messageData, signingKey.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	secureMessage := &SecureMessage{
		ID:              uuid.New().String(),
		OriginalMessage: message,
		Signature:       signature,
		SigningKey:      signingKeyID,
		Timestamp:       time.Now(),
		Metadata:        make(map[string]interface{}),
	}

	csm.logger.WithFields(map[string]interface{}{
		"secure_message_id": secureMessage.ID,
		"original_message_id": message.ID,
		"signing_key_id": signingKeyID,
	}).Debug("Message signed successfully")

	return secureMessage, nil
}

// VerifyMessage verifies a signed message
func (csm *CommunicationSecurityManager) VerifyMessage(ctx context.Context, secureMessage *SecureMessage) (*Message, error) {
	csm.mu.RLock()
	signingKey, exists := csm.signingKeys[secureMessage.SigningKey]
	csm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("signing key %s not found", secureMessage.SigningKey)
	}

	if !signingKey.Active {
		return nil, fmt.Errorf("signing key %s is not active", secureMessage.SigningKey)
	}

	// Serialize message
	messageData, err := json.Marshal(secureMessage.OriginalMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize message: %w", err)
	}

	// Verify signature
	if err := csm.verifyWithRSA(messageData, secureMessage.Signature, signingKey.PublicKey); err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}

	csm.logger.WithFields(map[string]interface{}{
		"secure_message_id": secureMessage.ID,
		"original_message_id": secureMessage.OriginalMessage.ID,
		"signing_key_id": secureMessage.SigningKey,
	}).Debug("Message signature verified successfully")

	return secureMessage.OriginalMessage, nil
}

// signWithRSA signs data with RSA
func (csm *CommunicationSecurityManager) signWithRSA(data []byte, privateKeyPEM []byte) ([]byte, error) {
	// Parse private key
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Hash data
	hasher := sha256.New()
	hasher.Write(data)
	hashedData := hasher.Sum(nil)

	// Sign data
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashedData)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	return signature, nil
}

// verifyWithRSA verifies data with RSA
func (csm *CommunicationSecurityManager) verifyWithRSA(data []byte, signature []byte, publicKeyPEM []byte) error {
	// Parse public key
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return fmt.Errorf("failed to parse PEM block")
	}

	publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	// Hash data
	hasher := sha256.New()
	hasher.Write(data)
	hashedData := hasher.Sum(nil)

	// Verify signature
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashedData, signature); err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	return nil
}

// =============================================================================
// TRUST MANAGEMENT
// =============================================================================

// AddTrustedAgent adds a trusted agent
func (csm *CommunicationSecurityManager) AddTrustedAgent(agentID string, agentType AgentType, publicKey []byte, trustLevel TrustLevel) (*TrustedAgent, error) {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	trustedAgent := &TrustedAgent{
		ID:         uuid.New().String(),
		AgentID:    agentID,
		AgentType:  agentType,
		PublicKey:  publicKey,
		TrustLevel: trustLevel,
		CreatedAt:  time.Now(),
		LastSeen:   time.Now(),
		Active:     true,
		Metadata:   make(map[string]interface{}),
	}

	csm.trustedAgents[trustedAgent.ID] = trustedAgent

	csm.logger.WithFields(map[string]interface{}{
		"trusted_agent_id": trustedAgent.ID,
		"agent_id":         agentID,
		"agent_type":       agentType,
		"trust_level":      trustLevel,
	}).Info("Trusted agent added successfully")

	return trustedAgent, nil
}

// RemoveTrustedAgent removes a trusted agent
func (csm *CommunicationSecurityManager) RemoveTrustedAgent(trustedAgentID string) error {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	delete(csm.trustedAgents, trustedAgentID)

	csm.logger.WithField("trusted_agent_id", trustedAgentID).Info("Trusted agent removed successfully")
	return nil
}

// IsAgentTrusted checks if an agent is trusted
func (csm *CommunicationSecurityManager) IsAgentTrusted(agentID string) (bool, *TrustedAgent) {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

	for _, trustedAgent := range csm.trustedAgents {
		if trustedAgent.AgentID == agentID && trustedAgent.Active {
			return true, trustedAgent
		}
	}

	return false, nil
}

// =============================================================================
// SECURITY POLICIES
// =============================================================================

// AddSecurityPolicy adds a security policy
func (csm *CommunicationSecurityManager) AddSecurityPolicy(policy *SecurityPolicy) error {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	policy.ID = uuid.New().String()
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	policy.Active = true

	csm.securityPolicies[policy.ID] = policy

	csm.logger.WithFields(map[string]interface{}{
		"policy_id":   policy.ID,
		"policy_name": policy.Name,
		"rule_count":  len(policy.Rules),
	}).Info("Security policy added successfully")

	return nil
}

// RemoveSecurityPolicy removes a security policy
func (csm *CommunicationSecurityManager) RemoveSecurityPolicy(policyID string) error {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	delete(csm.securityPolicies, policyID)

	csm.logger.WithField("policy_id", policyID).Info("Security policy removed successfully")
	return nil
}

// EvaluateSecurityPolicy evaluates a security policy
func (csm *CommunicationSecurityManager) EvaluateSecurityPolicy(ctx context.Context, message *Message) (SecurityAction, error) {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

	// Evaluate all active policies
	for _, policy := range csm.securityPolicies {
		if !policy.Active {
			continue
		}

		// Evaluate policy rules
		for _, rule := range policy.Rules {
			if !rule.Active {
				continue
			}

			// Check if rule condition matches
			if csm.matchesSecurityRule(message, rule) {
				csm.logger.WithFields(map[string]interface{}{
					"policy_id": policy.ID,
					"rule_id":   rule.ID,
					"action":    rule.Action,
					"message_id": message.ID,
				}).Debug("Security rule matched")

				return rule.Action, nil
			}
		}
	}

	// Default action if no rules match
	return SecurityActionAllow, nil
}

// matchesSecurityRule checks if a message matches a security rule
func (csm *CommunicationSecurityManager) matchesSecurityRule(message *Message, rule *SecurityRule) bool {
	// This is a simplified implementation
	// In a real system, this would evaluate the rule condition against the message
	
	switch rule.Type {
	case SecurityRuleTypeEncryption:
		// Check if message should be encrypted
		return message.Priority >= MessagePriorityHigh
	case SecurityRuleTypeSigning:
		// Check if message should be signed
		return message.Type == "request" || message.Type == "response"
	case SecurityRuleTypeAccess:
		// Check access permissions
		return true // Simplified
	case SecurityRuleTypeRateLimit:
		// Check rate limiting
		return true // Simplified
	default:
		return false
	}
}

// =============================================================================
// STATUS AND MONITORING
// =============================================================================

// GetStatus returns the current status of the security manager
func (csm *CommunicationSecurityManager) GetStatus() map[string]interface{} {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

	return map[string]interface{}{
		"encryption_keys":   len(csm.encryptionKeys),
		"signing_keys":      len(csm.signingKeys),
		"trusted_agents":    len(csm.trustedAgents),
		"security_policies": len(csm.securityPolicies),
		"timestamp":         time.Now().Format(time.RFC3339),
	}
}

// GetTrustedAgents returns the list of trusted agents
func (csm *CommunicationSecurityManager) GetTrustedAgents() []*TrustedAgent {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

	agents := make([]*TrustedAgent, 0, len(csm.trustedAgents))
	for _, agent := range csm.trustedAgents {
		agents = append(agents, agent)
	}

	return agents
}

// GetSecurityPolicies returns the list of security policies
func (csm *CommunicationSecurityManager) GetSecurityPolicies() []*SecurityPolicy {
	csm.mu.RLock()
	defer csm.mu.RUnlock()

	policies := make([]*SecurityPolicy, 0, len(csm.securityPolicies))
	for _, policy := range csm.securityPolicies {
		policies = append(policies, policy)
	}

	return policies
}
