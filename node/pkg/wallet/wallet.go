package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"
)

// Status constants
const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)

// Wallet holds an Ed25519 keypair for node identity.
type Wallet struct {
	ID        int64  // DB row ID, 0 if not saved yet
	Seed      []byte // 32 bytes — the canonical secret
	PublicKey []byte // 32 bytes — derived from seed
	Name      string
	Status    string
}

type walletRow struct {
	ID            int64
	PublicKey     string
	SeedEncrypted []byte
	Nonce         []byte
	CreatedAt     int64
	Name          string
	Status        string
}

const keyLen = 32 // AES-256

// Generate creates a new Ed25519 keypair.
func Generate() (*Wallet, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("generate seed: %w", err)
	}
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	return &Wallet{Seed: seed, PublicKey: pub, Status: StatusActive}, nil
}

// LoadFromDB reads the first active wallet from the database.
func LoadFromDB(d *sql.DB) (*Wallet, error) {
	var r walletRow
	err := d.QueryRow(
		`SELECT id, public_key, seed_encrypted, nonce, created_at, COALESCE(name,''), COALESCE(status,'active') FROM wallet WHERE status = ? ORDER BY id ASC LIMIT 1`,
		StatusActive,
	).Scan(&r.ID, &r.PublicKey, &r.SeedEncrypted, &r.Nonce, &r.CreatedAt, &r.Name, &r.Status)
	if err != nil {
		return nil, err
	}

	seed, err := decryptSeed(r.SeedEncrypted, r.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decrypt wallet: %w", err)
	}

	pub, err := hex.DecodeString(r.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}

	return &Wallet{ID: r.ID, Seed: seed, PublicKey: pub, Name: r.Name, Status: r.Status}, nil
}

// LoadAnyFromDB loads any wallet regardless of status (for migration purposes).
func LoadAnyFromDB(d *sql.DB) (*Wallet, error) {
	var r walletRow
	err := d.QueryRow(
		`SELECT id, public_key, seed_encrypted, nonce, created_at, COALESCE(name,''), COALESCE(status,'active') FROM wallet ORDER BY id ASC LIMIT 1`,
	).Scan(&r.ID, &r.PublicKey, &r.SeedEncrypted, &r.Nonce, &r.CreatedAt, &r.Name, &r.Status)
	if err != nil {
		return nil, err
	}

	seed, err := decryptSeed(r.SeedEncrypted, r.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decrypt wallet: %w", err)
	}

	pub, err := hex.DecodeString(r.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}

	return &Wallet{ID: r.ID, Seed: seed, PublicKey: pub, Name: r.Name, Status: r.Status}, nil
}

// Save inserts the wallet into the database (idempotent — INSERT only if no wallet exists).
func (w *Wallet) Save(d *sql.DB) error {
	seedEncrypted, nonce, err := encryptSeed(w.Seed)
	if err != nil {
		return fmt.Errorf("encrypt seed: %w", err)
	}

	now := time.Now().Unix()
	pubHex := hex.EncodeToString(w.PublicKey)
	name := w.Name
	if name == "" {
		name = "default"
	}
	status := w.Status
	if status == "" {
		status = StatusActive
	}

	result, err := d.Exec(
		`INSERT INTO wallet(public_key, seed_encrypted, nonce, created_at, name, status) VALUES (?, ?, ?, ?, ?, ?)`,
		pubHex, seedEncrypted, nonce, now, name, status)
	if err != nil {
		return fmt.Errorf("save wallet: %w", err)
	}

	id, _ := result.LastInsertId()
	w.ID = id
	return nil
}

// MarkInactive sets all wallets' status to inactive.
func MarkInactive(d *sql.DB) error {
	_, err := d.Exec(`UPDATE wallet SET status = ? WHERE status = ?`, StatusInactive, StatusActive)
	return err
}

// ExistsInDB returns true if at least one wallet record exists.
func ExistsInDB(d *sql.DB) bool {
	var count int
	d.QueryRow("SELECT COUNT(*) FROM wallet").Scan(&count)
	return count > 0
}

// HasActiveWallet returns true if an active wallet exists.
func HasActiveWallet(d *sql.DB) bool {
	var count int
	d.QueryRow("SELECT COUNT(*) FROM wallet WHERE status = ?", StatusActive).Scan(&count)
	return count > 0
}

// Sign signs a message with the wallet's private key.
func (w *Wallet) Sign(msg []byte) []byte {
	priv := ed25519.NewKeyFromSeed(w.Seed)
	return ed25519.Sign(priv, msg)
}

// Verify checks a signature against the wallet's public key.
func (w *Wallet) Verify(msg, sig []byte) bool {
	return ed25519.Verify(w.PublicKey, msg, sig)
}

// Address returns the wallet address: hex(public_key).
func (w *Wallet) Address() string {
	return hex.EncodeToString(w.PublicKey)
}

// SeedHex returns the seed as hex (for backup display).
func (w *Wallet) SeedHex() string {
	return hex.EncodeToString(w.Seed)
}

// HostInfo returns machine hostname + OS (for diagnostics).
func HostInfo() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		h = "unknown"
	}
	return fmt.Sprintf("%s (%s)", h, runtime.GOOS)
}

// ---- MAC-based key derivation ----

func deriveKey() []byte {
	mac := macAddress()
	h := sha256.Sum256([]byte(mac))
	return h[:]
}

func macAddress() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return fallbackMachineID()
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		if isVirtual(iface.Name) {
			continue
		}
		return iface.HardwareAddr.String()
	}
	return fallbackMachineID()
}

func isVirtual(name string) bool {
	virtualPrefixes := []string{"veth", "docker", "br-", "vnic", "vmnet", "vbox", "virbr"}
	for _, p := range virtualPrefixes {
		if len(name) >= len(p) && name[:len(p)] == p {
			return true
		}
	}
	return false
}

func fallbackMachineID() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "flux-node-default-key"
	}
	return h
}

// ---- encryption helpers ----

func encryptSeed(seed []byte) (ciphertext, nonce []byte, err error) {
	key := deriveKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("gcm: %w", err)
	}
	nonce = make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("nonce: %w", err)
	}
	ciphertext = aead.Seal(nil, nonce, seed, nil)
	return ciphertext, nonce, nil
}

func decryptSeed(ciphertext, nonce []byte) ([]byte, error) {
	key := deriveKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	seed, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return seed, nil
}
