package vaultenv

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/kubernetes"
	"github.com/james-lawrence/vaultenv/internal/x/errorsx"
	"github.com/james-lawrence/vaultenv/internal/x/stringsx"
	"github.com/pkg/errors"
)

// VaultDefaultTokenPath returns the path to the default location of the vault token.
func vaultDefaultTokenPath() string {
	var (
		err error
		u   *user.User
	)

	if u, err = user.Current(); err != nil {
		log.Println("failed to lookup the current user, unable to generate default token path", err)
		return ""
	}

	return filepath.Join(u.HomeDir, ".vault-token")
}

func DetectAuth() api.AuthMethod {
	switch os.Getenv("VAULTENV_AUTH_METHOD") {
	case "k8s":
		if a, err := kubernetes.NewKubernetesAuth(os.Getenv("VAULTENV_AUTH_K8S_ROLE_NAME")); err == nil {
			return a
		} else {
			log.Println("")
			return nil
		}
	default:
		return nil
	}
}

// NewVault configures vault with defaults.
func NewVault(auth api.AuthMethod) (v Vault, err error) {
	var (
		client *api.Client
		config *api.Config
	)

	if config = api.DefaultConfig(); config.Error != nil {
		return v, errors.WithStack(config.Error)
	}

	if client, err = api.NewClient(config); err != nil {
		return v, errors.WithStack(err)
	}

	if auth == nil {
		client.SetToken(stringsx.DefaultIfBlank(client.Token(), readTokenFile(vaultDefaultTokenPath())))
	} else {
		if _, err = client.Auth().Login(context.Background(), auth); err != nil {
			return v, errors.WithStack(err)
		}
	}

	return Vault{
		client: client,
	}, nil
}

// Vault api retrieves latest secrets from the vault
type Vault struct {
	client *api.Client
}

// Read
func (t Vault) Read(path string) (err error) {
	var (
		secret     *api.Secret
		ok         bool
		opaqueData interface{}
		data       map[string]interface{}
	)

	mountPath, v2, err := isKVv2(path, t.client)
	if err != nil {
		return err
	}

	if v2 {
		path = addPrefixToVKVPath(path, mountPath, "data")
	}

	if secret, err = kvReadRequest(t.client, path); err != nil {
		return err
	}

	if opaqueData, ok = secret.Data["data"]; !ok {
		return errorsx.String("missing secret")
	}

	if data, ok = opaqueData.(map[string]interface{}); !ok {
		return errorsx.String("invalid secret data")
	}

	for k, v := range data {
		var (
			value string
		)

		if value, ok = v.(string); !ok {
			return errorsx.String("values must be strings")
		}

		if err = os.Setenv(k, value); err != nil {
			return err
		}
	}

	return nil
}

func readTokenFile(path string) string {
	var (
		err error
		raw []byte
	)

	if raw, err = ioutil.ReadFile(path); err != nil {
		log.Println("failed to read vault token from file", path, err)
		return ""
	}

	return string(raw)
}

func kvReadRequest(client *api.Client, path string) (*api.Secret, error) {
	r := client.NewRequest("GET", "/v1/"+path)
	resp, err := client.RawRequest(r)
	if resp != nil {
		defer resp.Body.Close()
	}
	if resp != nil && resp.StatusCode == 404 {
		secret, parseErr := api.ParseSecret(resp.Body)
		switch parseErr {
		case nil:
		case io.EOF:
			return nil, nil
		default:
			return nil, err
		}
		if secret != nil && (len(secret.Warnings) > 0 || len(secret.Data) > 0) {
			return secret, nil
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return api.ParseSecret(resp.Body)
}

func isKVv2(path string, client *api.Client) (string, bool, error) {
	mountPath, version, err := kvPreflightVersionRequest(client, path)
	if err != nil {
		return "", false, err
	}

	return mountPath, version == 2, nil
}

func addPrefixToVKVPath(p, mountPath, apiPrefix string) string {
	switch {
	case p == mountPath, p == strings.TrimSuffix(mountPath, "/"):
		return path.Join(mountPath, apiPrefix)
	default:
		p = strings.TrimPrefix(p, mountPath)
		return path.Join(mountPath, apiPrefix, p)
	}
}

func kvPreflightVersionRequest(client *api.Client, path string) (string, int, error) {
	// We don't want to use a wrapping call here so save any custom value and
	// restore after
	currentWrappingLookupFunc := client.CurrentWrappingLookupFunc()
	client.SetWrappingLookupFunc(nil)
	defer client.SetWrappingLookupFunc(currentWrappingLookupFunc)
	currentOutputCurlString := client.OutputCurlString()
	client.SetOutputCurlString(false)
	defer client.SetOutputCurlString(currentOutputCurlString)

	r := client.NewRequest("GET", "/v1/sys/internal/ui/mounts/"+path)
	resp, err := client.RawRequest(r)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		// If we get a 404 we are using an older version of vault, default to
		// version 1
		if resp != nil && resp.StatusCode == 404 {
			return "", 1, nil
		}

		return "", 0, err
	}

	secret, err := api.ParseSecret(resp.Body)
	if err != nil {
		return "", 0, err
	}
	var mountPath string
	if mountPathRaw, ok := secret.Data["path"]; ok {
		mountPath = mountPathRaw.(string)
	}
	options := secret.Data["options"]
	if options == nil {
		return mountPath, 1, nil
	}
	versionRaw := options.(map[string]interface{})["version"]
	if versionRaw == nil {
		return mountPath, 1, nil
	}
	version := versionRaw.(string)
	switch version {
	case "", "1":
		return mountPath, 1, nil
	case "2":
		return mountPath, 2, nil
	}

	return mountPath, 1, nil
}
