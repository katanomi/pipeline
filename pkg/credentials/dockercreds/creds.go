/*
Copyright 2019 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dockercreds

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tektoncd/pipeline/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
)

const annotationPrefix = "tekton.dev/docker-"

var config basicRegistry
var registryConfig arrayArg
var legacyCfgArgs arrayArg

// AddFlags adds CLI flags supported by the registry credential helper.
func AddFlags(flagSet *flag.FlagSet) {
	flags(flagSet)
}

func flags(fs *flag.FlagSet) {
	config = basicRegistry{make(map[string]entry)}
	registryConfig = arrayArg{[]string{}}
	legacyCfgArgs = arrayArg{[]string{}}
	fs.Var(&config, "basic-docker", "List of secret=url pairs.")
	fs.Var(&config, "basic-registry", "List of secret=url pairs.")
	fs.Var(&registryConfig, "docker-config", "Registry config.json secret file.")
	fs.Var(&registryConfig, "registry-config", "Registry config.json secret file.")
	fs.Var(&legacyCfgArgs, "docker-cfg", ".dockercfg secret file.")
	fs.Var(&legacyCfgArgs, "registry-cfg", ".dockercfg secret file.")
}

// As the flag is read, this status is populated.
// basicRegistry implements flag.Value
type basicRegistry struct {
	Entries map[string]entry `json:"auths"`
}

func (dc *basicRegistry) String() string {
	if dc == nil {
		// According to flag.Value this can happen.
		return ""
	}
	var urls []string
	for k, v := range dc.Entries {
		urls = append(urls, fmt.Sprintf("%s=%s", v.Secret, k))
	}
	return strings.Join(urls, ",")
}

// Set sets a secret for a URL from a value in the format of "secret=url"
func (dc *basicRegistry) Set(value string) error {
	parts := strings.Split(value, "=")
	if len(parts) != 2 {
		return fmt.Errorf("expect entries of the form secret=url, got: %v", value)
	}
	secret := parts[0]
	url := parts[1]

	e, err := newEntry(secret)
	if err != nil {
		return err
	}
	dc.Entries[url] = *e
	return nil
}

type arrayArg struct {
	Values []string
}

// Set adds a value to the arrayArg's value slice
func (aa *arrayArg) Set(value string) error {
	aa.Values = append(aa.Values, value)
	return nil
}

func (aa *arrayArg) String() string {
	return strings.Join(aa.Values, ",")
}

type configFile struct {
	Auth map[string]entry `json:"auths"`
}

type entry struct {
	Secret   string `json:"-"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth"`
	Email    string `json:"email,omitempty"`
}

func newEntry(secret string) (*entry, error) {
	secretPath := credentials.VolumeName(secret)

	ub, err := os.ReadFile(filepath.Join(secretPath, corev1.BasicAuthUsernameKey))
	if err != nil {
		return nil, err
	}
	username := string(ub)

	pb, err := os.ReadFile(filepath.Join(secretPath, corev1.BasicAuthPasswordKey))
	if err != nil {
		return nil, err
	}
	password := string(pb)

	return &entry{
		Secret:   secret,
		Username: username,
		Password: password,
		Auth:     base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password))),
		Email:    "not@val.id",
	}, nil
}

type basicRegistryBuilder struct{}

// NewBuilder returns a new builder for registry credentials.
func NewBuilder() credentials.Builder { return &basicRegistryBuilder{} }

// MatchingAnnotations extracts flags for the credential helper
// from the supplied secret and returns a slice (of length 0 or
// greater) of applicable domains.
func (*basicRegistryBuilder) MatchingAnnotations(secret *corev1.Secret) []string {
	var flags []string
	switch secret.Type {
	case corev1.SecretTypeBasicAuth:
		for _, v := range credentials.SortAnnotations(secret.Annotations, annotationPrefix) {
			flags = append(flags, fmt.Sprintf("-basic-docker=%s=%s", secret.Name, v))
		}
	case corev1.SecretTypeDockerConfigJson:
		flags = append(flags, fmt.Sprintf("-docker-config=%s", secret.Name))
	case corev1.SecretTypeDockercfg:
		flags = append(flags, fmt.Sprintf("-docker-cfg=%s", secret.Name))

	case corev1.SecretTypeOpaque, corev1.SecretTypeServiceAccountToken, corev1.SecretTypeSSHAuth, corev1.SecretTypeTLS, corev1.SecretTypeBootstrapToken:
		return flags

	default:
		return flags
	}

	return flags
}

// Write builds a registry config file (config.json under the config dir) from a combination
// of kubernetes registry secrets and tekton registry
// secret entries and writes it to the given directory. If
// no entries exist then nothing will be written to disk.
func (*basicRegistryBuilder) Write(directory string) error {
	registryConfigDir := filepath.Join(directory, ".docker")
	registryConfigFile := filepath.Join(registryConfigDir, "config.json")
	cf := configFile{Auth: config.Entries}
	auth := map[string]entry{}

	for _, secretName := range legacyCfgArgs.Values {
		registryCfgAuthMap, err := authsFromDockerCfg(secretName)
		if err != nil {
			return err
		}
		for k, v := range registryCfgAuthMap {
			auth[k] = v
		}
	}

	for _, secretName := range registryConfig.Values {
		registryConfigAuthMap, err := authsFromRegistryConfig(secretName)
		if err != nil {
			return err
		}
		for k, v := range registryConfigAuthMap {
			auth[k] = v
		}
	}
	for k, v := range config.Entries {
		auth[k] = v
	}
	if len(auth) == 0 {
		return nil
	}
	if err := os.MkdirAll(registryConfigDir, os.ModePerm); err != nil {
		return err
	}

	cf.Auth = auth
	content, err := json.Marshal(cf)
	if err != nil {
		return err
	}
	return os.WriteFile(registryConfigFile, content, 0600)
}

func authsFromDockerCfg(secret string) (map[string]entry, error) {
	secretPath := credentials.VolumeName(secret)
	m := make(map[string]entry)
	data, err := os.ReadFile(filepath.Join(secretPath, corev1.DockerConfigKey))
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(data, &m)
	return m, err
}

func authsFromRegistryConfig(secret string) (map[string]entry, error) {
	secretPath := credentials.VolumeName(secret)
	m := make(map[string]entry)
	c := configFile{}
	data, err := os.ReadFile(filepath.Join(secretPath, corev1.DockerConfigJsonKey))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return m, err
	}
	for k, v := range c.Auth {
		m[k] = v
	}
	return m, nil
}
