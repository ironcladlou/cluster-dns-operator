package manifests

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/yaml"

	dnsv1alpha1 "github.com/openshift/cluster-dns-operator/pkg/apis/dns/v1alpha1"
	"github.com/openshift/cluster-dns-operator/pkg/util"
)

const (
	ClusterDNSDefaultCR = "assets/cluster-dns-cr.yaml"

	DNSNamespace          = "assets/dns/namespace.yaml"
	DNSServiceAccount     = "assets/dns/service-account.yaml"
	DNSClusterRole        = "assets/dns/cluster-role.yaml"
	DNSClusterRoleBinding = "assets/dns/cluster-role-binding.yaml"
	DNSConfigMap          = "assets/dns/configmap.yaml"
	DNSDaemonSet          = "assets/dns/daemonset.yaml"
	DNSService            = "assets/dns/service.yaml"
)

func MustAssetReader(asset string) io.Reader {
	return bytes.NewReader(MustAsset(asset))
}

// Factory knows how to create dns-related cluster resources from manifest
// files. It provides a point of control to mutate the static resources with
// provided configuration.
type Factory struct {
}

func NewFactory() *Factory {
	return &Factory{}
}

func (f *Factory) ClusterDNSDefaultCR(cm *corev1.ConfigMap) (*dnsv1alpha1.ClusterDNS, error) {
	cr, err := NewClusterDNS(MustAssetReader(ClusterDNSDefaultCR))
	if err != nil {
		return nil, err
	}
	clusterIP, err := util.GetDefaultClusterDNSIP(cm)
	if err != nil {
		return nil, err
	}
	cr.Spec.ClusterIP = &clusterIP
	return cr, nil
}

func (f *Factory) DNSNamespace() (*corev1.Namespace, error) {
	ns, err := NewNamespace(MustAssetReader(DNSNamespace))
	if err != nil {
		return nil, err
	}
	return ns, nil
}

func (f *Factory) DNSServiceAccount() (*corev1.ServiceAccount, error) {
	sa, err := NewServiceAccount(MustAssetReader(DNSServiceAccount))
	if err != nil {
		return nil, err
	}
	return sa, nil
}

func (f *Factory) DNSClusterRole() (*rbacv1.ClusterRole, error) {
	cr, err := NewClusterRole(MustAssetReader(DNSClusterRole))
	if err != nil {
		return nil, err
	}
	return cr, nil
}

func (f *Factory) DNSClusterRoleBinding() (*rbacv1.ClusterRoleBinding, error) {
	crb, err := NewClusterRoleBinding(MustAssetReader(DNSClusterRoleBinding))
	if err != nil {
		return nil, err
	}
	return crb, nil
}

func (f *Factory) DNSConfigMap(dns *dnsv1alpha1.ClusterDNS) (*corev1.ConfigMap, error) {
	cm, err := NewConfigMap(MustAssetReader(DNSConfigMap))
	if err != nil {
		return nil, err
	}
	cm.Name = "dns-" + dns.Name

	if dns.Spec.ClusterDomain != nil {
		cm.Data["Corefile"] = strings.Replace(cm.Data["Corefile"], "cluster.local", *dns.Spec.ClusterDomain, -1)
	}
	return cm, nil
}

func (f *Factory) DNSDaemonSet(dns *dnsv1alpha1.ClusterDNS) (*appsv1.DaemonSet, error) {
	ds, err := NewDaemonSet(MustAssetReader(DNSDaemonSet))
	if err != nil {
		return nil, err
	}
	ds.Name = "dns-" + dns.Name

	if ds.Spec.Template.Labels == nil {
		ds.Spec.Template.Labels = map[string]string{}
	}
	ds.Spec.Template.Labels["dns"] = ds.Name

	if ds.Spec.Selector.MatchLabels == nil {
		ds.Spec.Selector.MatchLabels = map[string]string{}
	}
	ds.Spec.Selector.MatchLabels["dns"] = ds.Name

	coreFileVolumeFound := false
	for i := range ds.Spec.Template.Spec.Volumes {
		if ds.Spec.Template.Spec.Volumes[i].Name == "config-volume" {
			ds.Spec.Template.Spec.Volumes[i].ConfigMap.Name = ds.Name
			coreFileVolumeFound = true
			break
		}
	}
	if !coreFileVolumeFound {
		return nil, fmt.Errorf("volume 'config-volume' not found")
	}
	return ds, nil
}

func (f *Factory) DNSService(dns *dnsv1alpha1.ClusterDNS) (*corev1.Service, error) {
	s, err := NewService(MustAssetReader(DNSService))
	if err != nil {
		return nil, err
	}
	s.Name = "dns-" + dns.Name

	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	s.Labels["dns"] = s.Name

	if s.Spec.Selector == nil {
		s.Spec.Selector = map[string]string{}
	}
	s.Spec.Selector["dns"] = s.Name

	if dns.Spec.ClusterIP != nil {
		s.Spec.ClusterIP = *dns.Spec.ClusterIP
	}
	return s, nil
}

func NewServiceAccount(manifest io.Reader) (*corev1.ServiceAccount, error) {
	sa := corev1.ServiceAccount{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&sa); err != nil {
		return nil, err
	}
	return &sa, nil
}

func NewClusterRole(manifest io.Reader) (*rbacv1.ClusterRole, error) {
	cr := rbacv1.ClusterRole{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

func NewClusterRoleBinding(manifest io.Reader) (*rbacv1.ClusterRoleBinding, error) {
	crb := rbacv1.ClusterRoleBinding{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&crb); err != nil {
		return nil, err
	}
	return &crb, nil
}

func NewConfigMap(manifest io.Reader) (*corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&cm); err != nil {
		return nil, err
	}
	return &cm, nil
}

func NewDaemonSet(manifest io.Reader) (*appsv1.DaemonSet, error) {
	ds := appsv1.DaemonSet{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&ds); err != nil {
		return nil, err
	}
	return &ds, nil
}

func NewService(manifest io.Reader) (*corev1.Service, error) {
	s := corev1.Service{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func NewNamespace(manifest io.Reader) (*corev1.Namespace, error) {
	ns := corev1.Namespace{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&ns); err != nil {
		return nil, err
	}
	return &ns, nil
}

func NewDeployment(manifest io.Reader) (*appsv1.Deployment, error) {
	o := appsv1.Deployment{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&o); err != nil {
		return nil, err
	}
	return &o, nil
}

func NewClusterDNS(manifest io.Reader) (*dnsv1alpha1.ClusterDNS, error) {
	o := dnsv1alpha1.ClusterDNS{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&o); err != nil {
		return nil, err
	}
	return &o, nil
}

func NewCustomResourceDefinition(manifest io.Reader) (*apiextensionsv1beta1.CustomResourceDefinition, error) {
	crd := apiextensionsv1beta1.CustomResourceDefinition{}
	if err := yaml.NewYAMLOrJSONDecoder(manifest, 100).Decode(&crd); err != nil {
		return nil, err
	}
	return &crd, nil
}
