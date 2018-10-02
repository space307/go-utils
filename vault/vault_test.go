package vault

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/vault/api"
	credAppRole "github.com/hashicorp/vault/builtin/credential/approle"
	"github.com/hashicorp/vault/builtin/logical/transit"
	vaulthttp "github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/vault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	vc, err := New("")
	assert.NoError(t, err)
	assert.NotEmpty(t, vc.client)
}

func TestVaultClient_Login(t *testing.T) {
	var err error

	client, closer := testVaultServer(t)
	defer closer()

	secret, err := client.Logical().Write("auth/approle/role/role1/secret-id", nil)
	if err != nil {
		t.Fatal(err)
	}
	secretID := secret.Data["secret_id"].(string)

	secret, err = client.Logical().Read("auth/approle/role/role1/role-id")
	if err != nil {
		t.Fatal(err)
	}
	roleID := secret.Data["role_id"].(string)

	vc, err := New(client.Address())
	assert.NoError(t, err)

	err = vc.Login(roleID, secretID)
	assert.NoError(t, err)

	err = vc.Login("asdfsdfs", secretID)
	assert.Error(t, err)
}

func TestVaultClient_Read(t *testing.T) {
	client, closer := testVaultServer(t)
	defer closer()

	err := client.Sys().PutPolicy("my-policy",
		`path "secret/read/foo" {
		  capabilities = ["read"]
	}
	path "secret/read/bar" {
		capabilities = ["read"]	
	}`)
	require.NoError(t, err)

	secret, err := client.Logical().Write("auth/approle/role/role1/secret-id", nil)
	require.NoError(t, err)
	secretID := secret.Data["secret_id"].(string)

	secret, err = client.Logical().Read("auth/approle/role/role1/role-id")
	require.NoError(t, err)
	roleID := secret.Data["role_id"].(string)

	_, err = client.Logical().Write("secret/read/foo", map[string]interface{}{"foo": "bar"})
	require.NoError(t, err)

	vc, err := New(client.Address())
	require.NoError(t, err)

	err = vc.Login(roleID, secretID)
	require.NoError(t, err)

	value, err := vc.Read("secret/read/foo", "foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", value)

	value, err = vc.Read("secret/read/foo", "foo1")
	assert.EqualError(t, err, fmt.Errorf(`empty value`).Error())

	value, err = vc.Read("secret/read/foo1", "foo")
	assert.Contains(t, err.Error(), `permission denied`)

	value, err = vc.Read("secret/read/bar", "foo")
	assert.EqualError(t, err, fmt.Errorf(`empty secret`).Error())

	value, err = vc.Read("secret1/read/foo", "foo")
	assert.Contains(t, err.Error(), `permission denied`)
}

func TestVaultClient_ReadAll(t *testing.T) {
	client, closer := testVaultServer(t)
	defer closer()

	err := client.Sys().PutPolicy("my-policy",
		`
	path "secret/bar" {
		capabilities = ["read"]	
	}`)
	require.NoError(t, err)

	secret, err := client.Logical().Write("auth/approle/role/role1/secret-id", nil)
	require.NoError(t, err)
	secretID := secret.Data["secret_id"].(string)

	secret, err = client.Logical().Read("auth/approle/role/role1/role-id")
	require.NoError(t, err)
	roleID := secret.Data["role_id"].(string)

	_, err = client.Logical().Write("secret/bar", map[string]interface{}{"foo": "bar1", "key": "val"})
	require.NoError(t, err)

	vc, err := New(client.Address())
	require.NoError(t, err)

	err = vc.Login(roleID, secretID)
	require.NoError(t, err)

	//read all
	all, err := vc.ReadAll("secret/bar")
	assert.NoError(t, err)
	assert.Equal(t, "bar1", all["foo"])
	assert.Equal(t, "val", all["key"])

	//read one
	one, err := vc.Read("secret/bar", "key")
	assert.NoError(t, err)
	assert.Equal(t, "val", one)

	//error
	_, err = vc.ReadAll("secret/bar_none")
	assert.Error(t, err)

	//nil
	_, err = client.Logical().Delete("secret/bar")
	require.NoError(t, err)

	_, err = vc.ReadAll("secret/bar")
	assert.Error(t, err)
}

func TestVaultClient_Encrypt(t *testing.T) {

	client, closer := testVaultServer(t)
	defer closer()

	err := client.Sys().PutPolicy("my-policy",
		`path "transit/*" {
  capabilities = ["create", "read", "update"]
}
`)

	require.NoError(t, err)

	secret, err := client.Logical().Write("auth/approle/role/role1/secret-id", nil)
	require.NoError(t, err)
	secretID := secret.Data["secret_id"].(string)

	secret, err = client.Logical().Read("auth/approle/role/role1/role-id")
	require.NoError(t, err)
	roleID := secret.Data["role_id"].(string)

	vc, err := New(client.Address())
	require.NoError(t, err)

	err = vc.Login(roleID, secretID)
	require.NoError(t, err)

	const transitKey = "sample"
	if err := client.Sys().Mount("transit", &api.MountInput{
		Type: "transit",
	}); err != nil {
		t.Fatal(err)
	}

	err = vc.CreateTransitKey(transitKey)
	require.NoError(t, err)

	const testStringData = "some special data"
	data := base64.StdEncoding.EncodeToString([]byte(testStringData))

	encrypted, err := vc.EncryptData(transitKey, data)
	require.NoError(t, err)
	require.NotEmpty(t, encrypted)

	_, err = vc.EncryptData(transitKey+"bad/dsdsa", data)
	require.Error(t, err)

	decrypted, err := vc.DecryptData(transitKey, encrypted)
	require.NoError(t, err)
	require.NotEmpty(t, decrypted)

	_, err = vc.DecryptData(transitKey+"bad/kk", encrypted)
	require.Error(t, err)

	_, err = vc.DecryptData(transitKey, encrypted+"bad")
	require.Error(t, err)

	byteData, err := base64.StdEncoding.DecodeString(decrypted)
	require.NoError(t, err)
	require.Equal(t, string(byteData), testStringData)
}

func testVaultServer(t testing.TB) (*api.Client, func()) {
	var err error

	os.Setenv(api.EnvVaultInsecure, "true")

	coreConfig := &vault.CoreConfig{
		DisableMlock: true,
		DisableCache: true,
		Logger:       nil,
		CredentialBackends: map[string]logical.Factory{
			"approle": credAppRole.Factory,
		},
		LogicalBackends: map[string]logical.Factory{
			"transit": transit.Factory,
		},
	}

	cluster := vault.NewTestCluster(t, coreConfig, &vault.TestClusterOptions{
		HandlerFunc: vaulthttp.Handler,
	})

	cluster.Start()

	cores := cluster.Cores

	vault.TestWaitActive(t, cores[0].Core)

	client := cores[0].Client

	err = client.Sys().EnableAuthWithOptions("approle", &api.EnableAuthOptions{Type: "approle"})
	require.NoError(t, err)

	_, err = client.Logical().Write("auth/approle/role/role1", map[string]interface{}{
		"bind_secret_id": "true",
		"period":         "300",
		"policies":       []string{"my-policy", "bar"},
	})
	require.NoError(t, err)

	return client, func() { defer cluster.Cleanup() }
}
