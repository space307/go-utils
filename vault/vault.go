package vault

import (
	"fmt"

	"github.com/hashicorp/vault/api"
)

type Vault interface {
	Read(path, name string) (string, error)
	Login(roleID, secretID string) error
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
