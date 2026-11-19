//go:build !edgeagent

// Teleport
// Copyright (C) 2023 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package services

import (
	"cmp"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport/api/types"
)

// NewApplicationFromKubeService creates application resources from kubernetes service.
// It transforms service fields and annotations into appropriate Teleport app fields.
// Service labels are copied to app labels.
func NewApplicationFromKubeService(service corev1.Service, clusterName, protocol string, port corev1.ServicePort) (types.Application, error) {
	appURI := buildAppURI(protocol, GetServiceFQDN(service), service.GetAnnotations()[types.DiscoveryPathLabel], port.Port)

	rewriteConfig, err := getAppRewriteConfig(service.GetAnnotations())
	if err != nil {
		return nil, trace.Wrap(err, "could not get app rewrite config for the service")
	}

	appNameAnnotation := service.GetAnnotations()[types.DiscoveryAppNameLabel]
	appName, err := getAppName(service.GetName(), service.GetNamespace(), clusterName, port.Name, appNameAnnotation)
	if err != nil {
		return nil, trace.Wrap(err, "could not create app name for the service")
	}

	labels, err := getAppLabels(service.GetLabels(), clusterName)
	if err != nil {
		return nil, trace.Wrap(err, "could not get labels for the service")
	}

	app, err := types.NewAppV3(types.Metadata{
		Name: appName,
		Description: cmp.Or(
			getDescription(service.GetAnnotations()),
			fmt.Sprintf("Discovered application in Kubernetes cluster %q", clusterName),
		),
		Labels: labels,
	}, types.AppSpecV3{
		URI:                appURI,
		Rewrite:            rewriteConfig,
		InsecureSkipVerify: getTLSInsecureSkipVerify(service.GetAnnotations()),
		PublicAddr:         getPublicAddr(service.GetAnnotations()),
	})
	if err != nil {
		return nil, trace.Wrap(err, "could not create an app from Kubernetes service")
	}

	return app, nil
}

// GetServiceFQDN returns the fully qualified domain name for the service.
func GetServiceFQDN(service corev1.Service) string {
	// If service type is ExternalName it points to external DNS name, to keep correct
	// HOST for HTTP requests we return already final external DNS name.
	// https://kubernetes.io/docs/concepts/services-networking/service/#externalname
	if service.Spec.Type == corev1.ServiceTypeExternalName {
		return service.Spec.ExternalName
	}
	return fmt.Sprintf("%s.%s.svc.%s", service.GetName(), service.GetNamespace(), clusterDomainResolver())
}

func buildAppURI(protocol, serviceFQDN, path string, port int32) string {
	return (&url.URL{
		Scheme: protocol,
		Host:   fmt.Sprintf("%s:%d", serviceFQDN, port),
		Path:   path,
	}).String()
}

func getAppRewriteConfig(annotations map[string]string) (*types.Rewrite, error) {
	rewritePayload := annotations[types.DiscoveryAppRewriteLabel]
	if rewritePayload == "" {
		return nil, nil
	}

	rw := types.Rewrite{}
	reader := strings.NewReader(rewritePayload)
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, 32*1024)
	err := decoder.Decode(&rw)
	if err != nil {
		return nil, trace.Wrap(err, "failed decoding rewrite config")
	}

	return &rw, nil
}

func getDescription(annotations map[string]string) string {
	return annotations[types.DiscoveryDescription]
}

func getPublicAddr(annotations map[string]string) string {
	return annotations[types.DiscoveryPublicAddr]
}

func getTLSInsecureSkipVerify(annotations map[string]string) bool {
	val := annotations[types.DiscoveryAppInsecureSkipVerify]
	if val == "" {
		return false
	}
	return val == "true"
}

func getAppName(serviceName, namespace, clusterName, portName, nameAnnotation string) (string, error) {
	if nameAnnotation != "" {
		name := nameAnnotation
		if portName != "" {
			name = fmt.Sprintf("%s-%s", name, portName)
		}

		if len(validation.IsDNS1035Label(name)) > 0 {
			return "", trace.BadParameter(
				"application name %q must be a lower case valid DNS subdomain: https://goteleport.com/docs/enroll-resources/application-access/guides/connecting-apps/#application-name", name)
		}

		return name, nil
	}

	clusterName = strings.ReplaceAll(clusterName, ".", "-")
	if portName != "" {
		return fmt.Sprintf("%s-%s-%s-%s", serviceName, portName, namespace, clusterName), nil
	}
	return fmt.Sprintf("%s-%s-%s", serviceName, namespace, clusterName), nil
}

func getAppLabels(serviceLabels map[string]string, clusterName string) (map[string]string, error) {
	result := make(map[string]string, len(serviceLabels)+1)

	for k, v := range serviceLabels {
		if !types.IsValidLabelKey(k) {
			return nil, trace.BadParameter("invalid label key: %q", k)
		}

		result[k] = v
	}
	result[types.KubernetesClusterLabel] = clusterName

	return result, nil
}

var (
	// clusterDomainResolver is a function that resolves the cluster domain once and caches the result.
	// It's used to lazily resolve the cluster domain from the env var "TELEPORT_KUBE_CLUSTER_DOMAIN" or fallback to
	// a default value.
	// It's only used when agent is running in the Kubernetes cluster.
	clusterDomainResolver = sync.OnceValue[string](getClusterDomain)
)

const (
	// teleportKubeClusterDomain is the environment variable that specifies the cluster domain.
	teleportKubeClusterDomain = "TELEPORT_KUBE_CLUSTER_DOMAIN"
)

func getClusterDomain() string {
	if envDomain := os.Getenv(teleportKubeClusterDomain); envDomain != "" {
		return envDomain
	}
	return "cluster.local"
}
