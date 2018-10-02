package vault

import (
	"fmt"

	"github.com/hashicorp/vault/api"
)

type Vault interface {
	ReadAll(path string) (map[string]string, error)
	Read(path, name string) (string, error)
	Login(roleID, secretID string) error
	CreateTransitKey(key string) error
	EncryptData(key, data string) (string, error)
	DecryptData(key, encrypted string) (string, error)
}

type VaultClient struct {
	client *api.Client
}

func New(address string) (*VaultClient, error) {
	vault, err := api.NewClient(&api.Config{
		Address: address})
	if err != nil {
		return nil, err
	}

	err = vault.SetAddress(address)
	if err != nil {
		return nil, err
	}

	return &VaultClient{client: vault}, nil
}

func (vc *VaultClient) ReadAll(path string) (map[string]string, error) {
	secret, err := vc.client.Logical().Read(path)
	if err != nil {
		return nil, err
	}

	if secret == nil {
		return nil, fmt.Errorf(`empty secret`)
	}

	var data = make(map[string]string, len(secret.Data))
	for key, val := range secret.Data {
		data[key] = val.(string)
	}

	return data, nil
}

func (vc *VaultClient) Read(path, name string) (string, error) {
	secret, err := vc.client.Logical().Read(path)
	if err != nil {
		return "", err
	}

	if secret == nil {
		return "", fmt.Errorf(`empty secret`)
	}

	val, ok := secret.Data[name]
	if !ok {
		return "", fmt.Errorf(`empty value`)
	}

	return val.(string), nil
}

func (vc *VaultClient) Login(roleID, secretID string) error {
	secret, err := vc.client.Logical().Write("auth/approle/login",
		map[string]interface{}{
			"role_id": roleID, "secret_id": secretID,
		})

	if err != nil {
		return err
	}

	if secret.Auth.ClientToken == "" {
		return fmt.Errorf("expected a successful login")
	}

	vc.client.SetToken(secret.Auth.ClientToken)

	return nil
}

func (vc *VaultClient) CreateTransitKey(key string) error {
	_, err := vc.client.Logical().Write("transit/keys/"+key, map[string]interface{}{})

	if err != nil {
		return err
	}

	return nil
}

func (vc *VaultClient) EncryptData(key, data string) (string, error) {
	secret, err := vc.client.Logical().Write("transit/encrypt/"+key,
		map[string]interface{}{
			"plaintext": data,
		})

	if err != nil {
		return "", err
	}

	encypted, ok := secret.Data["ciphertext"]
	if !ok {
		return "", fmt.Errorf("expected encrypted data!")

	}

	return encypted.(string), nil
}

func (vc *VaultClient) DecryptData(key, encrypted string) (string, error) {
	secret, err := vc.client.Logical().Write("transit/decrypt/"+key,
		map[string]interface{}{
			"ciphertext": encrypted,
		})

	if err != nil {
		return "", err
	}

	decrypted, ok := secret.Data["plaintext"]
	if !ok {
		return "", fmt.Errorf("expected decrypted data!")

	}

	return decrypted.(string), nil
}
