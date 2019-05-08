package location

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"net/http"

	"github.com/ovotech/cloud-key-rotator/pkg/cred"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gkev1 "google.golang.org/api/container/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

//K8s type
type K8s struct {
	Project     string
	Location    string
	ClusterName string
	Namespace   string
	SecretName  string
	DataName    string
}

//googleAuthProvider type
type googleAuthProvider struct {
	tokenSource oauth2.TokenSource
}

var (
	googleScopes = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email"}
	_ rest.AuthProvider = &googleAuthProvider{}
	// logger                   = log.StdoutLogger()
)

const googleAuthPlugin = "google" // so that this is different than "gcp" that's already in client-go tree.

func init() {
	if err := rest.RegisterAuthProviderPlugin(googleAuthPlugin, newGoogleAuthProvider); err != nil {
		logger.Fatalf("Failed to register %s auth plugin: %v", googleAuthPlugin, err)
	}
}

func (k8s K8s) Write(serviceAccountName, keyID, key, keyProvider string, creds cred.Credentials) (updated UpdatedLocation, err error) {
	var cluster *gkev1.Cluster

	if cluster, err = gkeCluster(k8s.Project, k8s.Location, k8s.ClusterName); err != nil {
		return
	}

	var k8sClient *kubernetes.Clientset
	if k8sClient, err = kubernetesClient(cluster); err != nil {
		return
	}

	if _, err = updateK8sSecret(k8s.SecretName, k8s.DataName, k8s.Namespace, key, k8sClient); err != nil {
		return
	}

	updated = UpdatedLocation{
		LocationType: "K8S",
		LocationURI:  k8s.Project,
		LocationIDs:  []string{k8s.Location}}

	return
}

//kubernetesClient creates a kubernetes clientset
func kubernetesClient(cluster *gkev1.Cluster) (k8sclient *kubernetes.Clientset, err error) {
	var decodedClientCertificate []byte
	if decodedClientCertificate, err = b64.StdEncoding.
		DecodeString(cluster.MasterAuth.ClientCertificate); err != nil {
		return
	}
	var decodedClientKey []byte
	if decodedClientKey, err = b64.StdEncoding.
		DecodeString(cluster.MasterAuth.ClientKey); err != nil {
		return
	}
	var decodedClusterCaCertificate []byte
	if decodedClusterCaCertificate, err = b64.StdEncoding.
		DecodeString(cluster.MasterAuth.ClusterCaCertificate); err != nil {
		return
	}
	return kubernetes.NewForConfig(&rest.Config{
		Username: cluster.MasterAuth.Username,
		Password: cluster.MasterAuth.Password,
		Host:     "https://" + cluster.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: false,
			CertData: decodedClientCertificate,
			KeyData:  decodedClientKey,
			CAData:   decodedClusterCaCertificate,
		},
		AuthProvider: &clientcmdapi.AuthProviderConfig{Name: googleAuthPlugin},
	})
}

func (g *googleAuthProvider) WrapTransport(rt http.RoundTripper) http.RoundTripper {
	return &oauth2.Transport{
		Base:   rt,
		Source: g.tokenSource,
	}
}

func (g *googleAuthProvider) Login() error { return nil }

func newGoogleAuthProvider(addr string, config map[string]string,
	persister rest.AuthProviderConfigPersister) (authProvider rest.AuthProvider, err error) {
	var ts oauth2.TokenSource
	if ts, err = google.DefaultTokenSource(context.TODO(), googleScopes...); err != nil {
		return
	}
	return &googleAuthProvider{tokenSource: ts}, nil
}

//updateK8sSecret updates a specific namespace/secret/data with the key string
func updateK8sSecret(secretName, dataName, namespace, key string,
	k8sclient *kubernetes.Clientset) (newSecret *v1.Secret, err error) {
	logger.Info("Starting k8s secret updates")
	var secret *v1.Secret
	if secret, err = k8sclient.CoreV1().Secrets(namespace).Get(secretName,
		metav1.GetOptions{}); err != nil {
		return
	}
	var decodedKey []byte
	if decodedKey, err = b64.StdEncoding.DecodeString(key); err != nil {
		return
	}
	secret.Data = map[string][]byte{dataName: decodedKey}
	return k8sclient.CoreV1().Secrets(namespace).Update(secret)
}

//gkeCluster creates a GKE cluster struct
func gkeCluster(project, location, clusterName string) (cluster *gkev1.Cluster, err error) {
	ctx := context.Background()
	var httpClient *http.Client
	if httpClient, err = google.DefaultClient(ctx, gkev1.CloudPlatformScope); err != nil {
		return
	}
	var gkeService *gkev1.Service
	if gkeService, err = gkev1.New(httpClient); err != nil {
		return
	}
	cluster, err = gkeService.Projects.Locations.Clusters.
		Get(fmt.Sprintf("projects/%s/locations/%s/clusters/%s", project, location, clusterName)).
		Do()
	return
}
