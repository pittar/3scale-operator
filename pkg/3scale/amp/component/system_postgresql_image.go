package component

import (
	"github.com/3scale/3scale-operator/pkg/common"

	imagev1 "github.com/openshift/api/image/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SystemPostgreSQLImage struct {
	Options *SystemPostgreSQLImageOptions
}

func NewSystemPostgreSQLImage(options *SystemPostgreSQLImageOptions) *SystemPostgreSQLImage {
	return &SystemPostgreSQLImage{Options: options}
}

func (s *SystemPostgreSQLImage) Objects() []common.KubernetesObject {
	systemPostgreSQLImageStream := s.buildSystemPostgreSQLImageStream()

	objects := []common.KubernetesObject{
		systemPostgreSQLImageStream,
	}

	return objects
}

func (s *SystemPostgreSQLImage) buildSystemPostgreSQLImageStream() *imagev1.ImageStream {
	return &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: "system-postgresql",
			Labels: map[string]string{
				"app":                  s.Options.appLabel,
				"threescale_component": "system",
			},
			Annotations: map[string]string{
				"openshift.io/display-name": "System database",
			},
		},
		TypeMeta: metav1.TypeMeta{APIVersion: "image.openshift.io/v1", Kind: "ImageStream"},
		Spec: imagev1.ImageStreamSpec{
			Tags: []imagev1.TagReference{
				imagev1.TagReference{
					Name: "latest",
					Annotations: map[string]string{
						"openshift.io/display-name": "System PostgreSQL (latest)",
					},
					From: &v1.ObjectReference{
						Kind: "ImageStreamTag",
						Name: s.Options.ampRelease,
					},
				},
				imagev1.TagReference{
					Name: s.Options.ampRelease,
					Annotations: map[string]string{
						"openshift.io/display-name": "System " + s.Options.ampRelease + " PostgreSQL",
					},
					From: &v1.ObjectReference{
						Kind: "DockerImage",
						Name: s.Options.image,
					},
					ImportPolicy: imagev1.TagImportPolicy{
						Insecure: s.Options.insecureImportPolicy,
					},
				},
			},
		},
	}
}