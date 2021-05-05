package actions

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	helmactions "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

var (
	defaultCacheDir = filepath.Join(homedir.HomeDir(), ".kube", "http-cache")
	ErrEmptyConfig  = errors.New(`Missing or incomplete configuration info.  Please point to an existing, complete config file:

	1. Via the command-line flag --kubeconfig
	2. Via the KUBECONFIG environment variable
	3. In your home directory as ~/.kube/config
  
  To view or setup config directly use the 'config' command.`)
)

type KubernetesClient struct {
	ConfigFilePath string
	ConfigString   string
	Namespace      string
	ClientName     string
	Config         *rest.Config
	Log            func(string, ...interface{})
}

type Option func(k *KubernetesClient)

func WithNamespace(namespace string) Option {
	return func(k *KubernetesClient) {
		k.Namespace = namespace
	}
}

func WithRestConfig(config *rest.Config) Option {
	return func(k *KubernetesClient) {
		k.Config = config
	}
}

func WithConfigFile(configFilePath string) Option {
	return func(k *KubernetesClient) {
		k.ConfigFilePath = configFilePath
	}
}

func WithConfigString(configString string) Option {
	return func(k *KubernetesClient) {
		k.ConfigString = configString
	}
}

func WithClientName(clientName string) Option {
	return func(k *KubernetesClient) {
		k.ClientName = clientName
	}
}

func NewKubernetesClient(opts ...Option) *KubernetesClient {
	kubernetesClient := &KubernetesClient{}
	for _, fn := range opts {
		fn(kubernetesClient)
	}
	return kubernetesClient
}

var _ genericclioptions.RESTClientGetter = &KubernetesClient{}

// ToRESTConfig returns restconfig
func (t *KubernetesClient) ToRESTConfig() (*rest.Config, error) {
	config, err := t.ToRawKubeConfigLoader().ClientConfig()
	// replace client-go's ErrEmptyConfig error with our custom, more verbose version
	if clientcmd.IsEmptyConfig(err) {
		return nil, ErrEmptyConfig
	}
	return config, err
}

// ToDiscoveryClient returns discovery client
func (t *KubernetesClient) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {

	config, err := t.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	// The more groups you have, the more discovery requests you need to make.
	// given 25 groups (our groups + a few custom resources) with one-ish version each, discovery needs to make 50 requests
	// double it just so we don't end up here again for a while.  This config is only used for discovery.
	config.Burst = 100

	// retrieve a user-provided value for the "cache-dir"
	// defaulting to ~/.kube/http-cache if no user-value is given.
	httpCacheDir := defaultCacheDir
	discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube", "cache", "discovery"), config.Host)
	if t.ClientName != "" {
		httpCacheDir = filepath.Join(homedir.HomeDir(), ".kube", "http-cache", t.ClientName)
		discoveryCacheDir = computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube", "cache", "discovery", t.ClientName), config.Host)
	}
	return diskcached.NewCachedDiscoveryClientForConfig(config, discoveryCacheDir, httpCacheDir, time.Duration(10*time.Minute))

}

// ToRESTMapper returns a restmapper
func (t *KubernetesClient) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := t.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

// ToRawKubeConfigLoader return kubeconfig loader as-is
func (t *KubernetesClient) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	if t.ConfigString != "" {
		config, err := clientcmd.Load([]byte(t.ConfigString))
		if err == nil {
			result := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{Context: clientcmdapi.Context{Namespace: t.Namespace}})
			return result
		}
	}
	if t.ConfigFilePath != "" {
		config, err := clientcmd.LoadFromFile(t.ConfigFilePath)
		if err == nil {
			return clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{Context: clientcmdapi.Context{Namespace: t.Namespace}})
		}
	}
	if t.Config != nil {
		var apiConfig = t.ConvertRestConfigToAPIConfig(t.Config)
		return clientcmd.NewDefaultClientConfig(apiConfig, &clientcmd.ConfigOverrides{Context: clientcmdapi.Context{Namespace: t.Namespace}})
	}
	//not test for this
	config, err := rest.InClusterConfig()
	if err == nil {
		var apiConfig = t.ConvertRestConfigToAPIConfig(config)
		return clientcmd.NewDefaultClientConfig(apiConfig, &clientcmd.ConfigOverrides{Context: clientcmdapi.Context{Namespace: t.Namespace}})
	}

	return &clientcmd.DirectClientConfig{}
}

func (t *KubernetesClient) ConvertRestConfigToAPIConfig(restConfig *rest.Config) clientcmdapi.Config {
	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters["default-cluster"] = &clientcmdapi.Cluster{
		Server:                   restConfig.Host,
		CertificateAuthorityData: restConfig.CAData,
	}

	contexts := make(map[string]*clientcmdapi.Context)
	contexts["default-context"] = &clientcmdapi.Context{
		Cluster:  "default-cluster",
		AuthInfo: "default-user",
	}

	var authInfo = &clientcmdapi.AuthInfo{}

	if restConfig.BearerToken != "" {
		authInfo.Token = restConfig.BearerToken
	}
	if restConfig.BearerTokenFile != "" {
		authInfo.TokenFile = restConfig.BearerTokenFile
	}
	if restConfig.Impersonate.UserName != "" {
		authInfo.Impersonate = restConfig.Impersonate.UserName
	}
	if len(restConfig.Impersonate.Groups) > 0 {
		authInfo.ImpersonateGroups = restConfig.Impersonate.Groups
	}
	if len(restConfig.Impersonate.Extra) > 0 {
		authInfo.ImpersonateUserExtra = restConfig.Impersonate.Extra
	}
	authInfo.ClientCertificate = restConfig.CertFile
	authInfo.ClientCertificateData = restConfig.CertData
	authInfo.ClientKey = restConfig.KeyFile
	authInfo.ClientKeyData = restConfig.KeyData
	if restConfig.Username != "" && restConfig.Password != "" {
		authInfo.Username = restConfig.Username
		authInfo.Password = restConfig.Password
	}
	if restConfig.AuthProvider != nil {
		authInfo.AuthProvider = restConfig.AuthProvider
	}
	if restConfig.ExecProvider != nil {
		authInfo.Exec = restConfig.ExecProvider
	}

	authinfos := make(map[string]*clientcmdapi.AuthInfo)
	authinfos["default-user"] = authInfo

	clientConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: "default-context",
		AuthInfos:      authinfos,
	}

	return clientConfig
}

func (t *KubernetesClient) GetHelmActionConfiguration(namespace string) (*helmactions.Configuration, error) {
	var cfg = &helmactions.Configuration{}
	if namespace != "" {
		t.Namespace = namespace
	}
	var log func(message string, formats ...interface{})
	if t.Log != nil {
		log = t.Log
	} else {
		log = func(message string, formats ...interface{}) {}
	}

	err := cfg.Init(t, t.Namespace, "secrets", log)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// overlyCautiousIllegalFileCharacters matches characters that *might* not be supported.  Windows is really restrictive, so this is really restrictive
var overlyCautiousIllegalFileCharacters = regexp.MustCompile(`[^(\w/\.)]`)

// computeDiscoverCacheDir takes the parentDir and the host and comes up with a "usually non-colliding" name.
func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if its there:
	schemelessHost := strings.Replace(strings.Replace(host, "https://", "", 1), "http://", "", 1)
	// now do a simple collapse of non-AZ09 characters.  Collisions are possible but unlikely.  Even if we do collide the problem is short lived
	safeHost := overlyCautiousIllegalFileCharacters.ReplaceAllString(schemelessHost, "_")
	return filepath.Join(parentDir, safeHost)
}

type InstallOptions struct {
	// ChartOpts install chart info
	ChartOpts *ChartOpts
	// ReleaseName install release name
	ReleaseName string
	// Values helm install values
	Values map[string]interface{}
	// KubernetesOptions kubernetes install Options
	KubernetesOptions *KubernetesClient
	// DryRun controls whether the operation is prepared, but not executed.
	// If `true`, the upgrade is prepared but not performed.
	DryRun bool
	// Description install custom description
	Description string
	// SkipCRDs is skip crd when install
	SkipCRDs bool
	// TimeOut time out time
	Timeout time.Duration
	// NoHook do not use hook
	NoHook       bool
	GenerateName bool
	//CreateNamespace create namespace when install
	CreateNamespace bool
	//DisableOpenAPIValidation disable openapi validation on kubernetes install
	DisableOpenAPIValidation bool
	IsUpgrade                bool
	WaitForJobs              bool
	Replace                  bool
	Wait                     bool
	Namespace                string
}

// Run  run install helm chart return helm release
func (i *InstallOptions) Run() (*release.Release, error) {
	cfg, err := i.KubernetesOptions.GetHelmActionConfiguration(i.Namespace)
	if err != nil {
		return nil, err
	}
	installConfig := helmactions.NewInstall(cfg)
	installConfig.SkipCRDs = i.SkipCRDs
	installConfig.DryRun = i.DryRun
	installConfig.RepoURL = i.ChartOpts.RepoOptions.RepoURL
	installConfig.CreateNamespace = i.CreateNamespace
	installConfig.Timeout = i.Timeout
	installConfig.DisableHooks = i.NoHook
	installConfig.DisableOpenAPIValidation = i.DisableOpenAPIValidation
	installConfig.Description = i.Description
	installConfig.GenerateName = i.GenerateName
	installConfig.ReleaseName = i.ReleaseName
	installConfig.IsUpgrade = i.IsUpgrade
	// helm 3.3.1 not support
	//installConfig.WaitForJobs = i.WaitForJobs
	installConfig.Replace = i.Replace
	installConfig.Wait = i.Wait
	installConfig.Namespace = i.Namespace

	chartInfo, err := i.ChartOpts.LoadChart()
	if err != nil {
		return nil, errors.Wrapf(err, "load chart from config error")
	}
	if i.Values == nil {
		i.Values = map[string]interface{}{}
	}
	return installConfig.Run(chartInfo, i.Values)
}
