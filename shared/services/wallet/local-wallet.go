package wallet

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/tyler-smith/go-bip39"
	eth2types "github.com/wealdtech/go-eth2-types/v2"
	eth2ks "github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4"

	"github.com/rocket-pool/smartnode/shared/services/passwords"
	"github.com/rocket-pool/smartnode/shared/services/wallet/keystore"
)

// Config
const (
	EntropyBits              = 256
	FileMode                 = 0600
	DefaultNodeKeyPath       = "m/44'/60'/0'/0/%d"
	LedgerLiveNodeKeyPath    = "m/44'/60'/%d/0/0"
	MyEtherWalletNodeKeyPath = "m/44'/60'/0'/%d"
)

// LocalWallet
type LocalWallet struct {
	// Core
	walletPath string
	pm         *passwords.PasswordManager
	encryptor  *eth2ks.Encryptor
	chainID    *big.Int

	// Encrypted store
	ws *localWalletStore

	// Seed & master key
	seed []byte
	mk   *hdkeychain.ExtendedKey

	// Node key cache
	nodeKey     *ecdsa.PrivateKey
	nodeKeyPath string

	// Validator key caches
	validatorKeys map[uint]*eth2types.BLSPrivateKey

	// Keystores
	keystores map[string]keystore.Keystore

	// Desired gas price & limit from config
	maxFee         *big.Int
	maxPriorityFee *big.Int
	gasLimit       uint64
}

// Encrypted wallet store
type localWalletStore struct {
	Crypto         map[string]interface{} `json:"crypto"`
	Name           string                 `json:"name"`
	Version        uint                   `json:"version"`
	UUID           uuid.UUID              `json:"uuid"`
	DerivationPath string                 `json:"derivationPath,omitempty"`
	WalletIndex    uint                   `json:"walletIndex,omitempty"`
	NextAccount    uint                   `json:"next_account"`
}

// Create new wallet
func NewLocalWallet(walletPath string, chainId uint, maxFee *big.Int, maxPriorityFee *big.Int, gasLimit uint64, passwordManager *passwords.PasswordManager) (*LocalWallet, error) {

	// Initialize wallet
	w := &LocalWallet{
		walletPath:     walletPath,
		pm:             passwordManager,
		encryptor:      eth2ks.New(),
		chainID:        big.NewInt(int64(chainId)),
		validatorKeys:  map[uint]*eth2types.BLSPrivateKey{},
		keystores:      map[string]keystore.Keystore{},
		maxFee:         maxFee,
		maxPriorityFee: maxPriorityFee,
		gasLimit:       gasLimit,
	}

	// Load & decrypt wallet store
	if _, err := w.loadStore(); err != nil {
		return nil, err
	}

	// Return
	return w, nil

}

// Gets the wallet's chain ID
func (w *LocalWallet) GetChainID() *big.Int {
	copy := big.NewInt(0).Set(w.chainID)
	return copy
}

// Add a keystore to the wallet
func (w *LocalWallet) AddKeystore(name string, ks keystore.Keystore) {
	w.keystores[name] = ks
}

// Check if the wallet has been initialized
func (w *LocalWallet) IsInitialized() bool {
	return (w.ws != nil && w.seed != nil && w.mk != nil)
}

// Attempt to initialize the wallet if not initialized and return status
func (w *LocalWallet) GetInitialized() (bool, error) {
	if w.IsInitialized() {
		return true, nil
	}
	return w.loadStore()
}

// Serialize the wallet to a JSON string
func (w *LocalWallet) String() (string, error) {

	// Check wallet is initialized
	if !w.IsInitialized() {
		return "", errors.New("Wallet is not initialized")
	}

	// Encode wallet store
	wsBytes, err := json.Marshal(w.ws)
	if err != nil {
		return "", fmt.Errorf("Could not encode wallet: %w", err)
	}

	// Return
	return string(wsBytes), nil

}

// Initialize the wallet from a random seed
func (w *LocalWallet) Initialize(derivationPath string, walletIndex uint) (string, error) {

	// Check wallet is not initialized
	if w.IsInitialized() {
		return "", errors.New("Wallet is already initialized")
	}

	// Generate mnemonic entropy
	entropy, err := bip39.NewEntropy(EntropyBits)
	if err != nil {
		return "", fmt.Errorf("Could not generate wallet mnemonic entropy bytes: %w", err)
	}

	// Generate mnemonic
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("Could not generate wallet mnemonic: %w", err)
	}

	// Initialize wallet store
	if err := w.initializeStore(derivationPath, walletIndex, mnemonic); err != nil {
		return "", err
	}

	// Return
	return mnemonic, nil

}

// Recover a wallet from a mnemonic
func (w *LocalWallet) Recover(derivationPath string, walletIndex uint, mnemonic string) error {

	// Check wallet is not initialized
	if w.IsInitialized() {
		return errors.New("Wallet is already initialized")
	}

	// Check mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		return fmt.Errorf("Invalid mnemonic '%s'", mnemonic)
	}

	// Initialize wallet store
	if err := w.initializeStore(derivationPath, walletIndex, mnemonic); err != nil {
		return err
	}

	// Return
	return nil

}

// Recover a wallet from a mnemonic - only used for testing mnemonics
func (w *LocalWallet) TestRecovery(derivationPath string, walletIndex uint, mnemonic string) error {

	// Check mnemonic
	if !bip39.IsMnemonicValid(mnemonic) {
		return fmt.Errorf("Invalid mnemonic '%s'", mnemonic)
	}

	// Generate seed
	w.seed = bip39.NewSeed(mnemonic, "")

	// Create master key
	var err error
	w.mk, err = hdkeychain.NewMaster(w.seed, &chaincfg.MainNetParams)
	if err != nil {
		return fmt.Errorf("Could not create wallet master key: %w", err)
	}

	// Create wallet store
	w.ws = &localWalletStore{
		Name:           w.encryptor.Name(),
		Version:        w.encryptor.Version(),
		UUID:           uuid.New(),
		DerivationPath: derivationPath,
		WalletIndex:    walletIndex,
		NextAccount:    0,
	}

	// Return
	return nil

}

// Save the wallet store to disk
func (w *LocalWallet) Save() error {

	// Check wallet is initialized
	if !w.IsInitialized() {
		return errors.New("Wallet is not initialized")
	}

	// Encode wallet store
	wsBytes, err := json.Marshal(w.ws)
	if err != nil {
		return fmt.Errorf("Could not encode wallet: %w", err)
	}

	// Write wallet store to disk
	if err := os.WriteFile(w.walletPath, wsBytes, FileMode); err != nil {
		return fmt.Errorf("Could not write wallet to disk: %w", err)
	}

	// Return
	return nil

}

// Delete the wallet store from disk
func (w *LocalWallet) Delete() error {

	// Check if it exists
	_, err := os.Stat(w.walletPath)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking wallet file path: %w", err)
	}

	// Write wallet store to disk
	err = os.Remove(w.walletPath)
	return err

}

// Signs a serialized TX using the wallet's private key
func (w *LocalWallet) Sign(serializedTx []byte) ([]byte, error) {
	// Get private key
	privateKey, _, err := w.getNodePrivateKey()
	if err != nil {
		return nil, err
	}

	tx := types.Transaction{}
	err = tx.UnmarshalBinary(serializedTx)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling TX: %w", err)
	}

	signer := types.NewLondonSigner(w.chainID)
	signedTx, err := types.SignTx(&tx, signer, privateKey)
	if err != nil {
		return nil, fmt.Errorf("Error signing TX: %w", err)
	}

	signedData, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("Error marshalling signed TX to binary: %w", err)
	}

	return signedData, nil
}

// Signs an arbitrary message using the wallet's private key
func (w *LocalWallet) SignMessage(message string) ([]byte, error) {
	// Get the wallet's private key
	privateKey, _, err := w.getNodePrivateKey()
	if err != nil {
		return nil, err
	}

	messageHash := accounts.TextHash([]byte(message))
	signedMessage, err := crypto.Sign(messageHash, privateKey)
	if err != nil {
		return nil, fmt.Errorf("Error signing message: %w", err)
	}

	// fix the ECDSA 'v' (see https://medium.com/mycrypto/the-magic-of-digital-signatures-on-ethereum-98fe184dc9c7#:~:text=The%20version%20number,2%E2%80%9D%20was%20introduced)
	signedMessage[crypto.RecoveryIDOffset] += 27
	return signedMessage, nil
}

// Reloads wallet from disk
func (w *LocalWallet) Reload() error {
	_, err := w.loadStore()
	return err
}

// Load the wallet store from disk and decrypt it
func (w *LocalWallet) loadStore() (bool, error) {

	// Read wallet store from disk; cancel if not found
	wsBytes, err := os.ReadFile(w.walletPath)
	if err != nil {
		return false, nil
	}

	// Decode wallet store
	w.ws = new(localWalletStore)
	if err = json.Unmarshal(wsBytes, w.ws); err != nil {
		return false, fmt.Errorf("Could not decode wallet: %w", err)
	}

	// Upgrade legacy wallets to include derivation paths
	if w.ws.DerivationPath == "" {
		w.ws.DerivationPath = DefaultNodeKeyPath
	}

	// Get wallet password
	password, err := w.pm.GetPassword()
	if err != nil {
		return false, fmt.Errorf("Could not get wallet password: %w", err)
	}

	// Decrypt seed
	w.seed, err = w.encryptor.Decrypt(w.ws.Crypto, password)
	if err != nil {
		return false, fmt.Errorf("Could not decrypt wallet seed: %w", err)
	}

	// Create master key
	w.mk, err = hdkeychain.NewMaster(w.seed, &chaincfg.MainNetParams)
	if err != nil {
		return false, fmt.Errorf("Could not create wallet master key: %w", err)
	}

	// Return
	return true, nil

}

// Initialize the encrypted wallet store from a mnemonic
func (w *LocalWallet) initializeStore(derivationPath string, walletIndex uint, mnemonic string) error {

	// Generate seed
	w.seed = bip39.NewSeed(mnemonic, "")

	// Create master key
	var err error
	w.mk, err = hdkeychain.NewMaster(w.seed, &chaincfg.MainNetParams)
	if err != nil {
		return fmt.Errorf("Could not create wallet master key: %w", err)
	}

	// Get wallet password
	password, err := w.pm.GetPassword()
	if err != nil {
		return fmt.Errorf("Could not get wallet password: %w", err)
	}

	// Encrypt seed
	encryptedSeed, err := w.encryptor.Encrypt(w.seed, password)
	if err != nil {
		return fmt.Errorf("Could not encrypt wallet seed: %w", err)
	}

	// Create wallet store
	w.ws = &localWalletStore{
		Crypto:         encryptedSeed,
		Name:           w.encryptor.Name(),
		Version:        w.encryptor.Version(),
		UUID:           uuid.New(),
		DerivationPath: derivationPath,
		WalletIndex:    walletIndex,
		NextAccount:    0,
	}

	// Return
	return nil

}
