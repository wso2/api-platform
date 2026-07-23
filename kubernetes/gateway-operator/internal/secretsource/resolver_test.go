package secretsource

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

func newClient(t *testing.T, secrets ...*corev1.Secret) (cl client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	objs := []runtime.Object{}
	for _, s := range secrets {
		objs = append(objs, s)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
}

func TestResolve_Inline(t *testing.T) {
	c := newClient(t)
	v := "hello"
	got, err := Resolve(context.Background(), c, "spec.value", apiv1.SecretValueSource{Value: &v}, "ns")
	require.NoError(t, err)
	require.Equal(t, "hello", got)
}

func TestResolve_SecretRef(t *testing.T) {
	c := newClient(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "team-a"},
		Data:       map[string][]byte{"token": []byte("from-secret")},
	})
	src := apiv1.SecretValueSource{
		ValueFrom: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "creds"},
			Key:                  "token",
		},
	}
	got, err := Resolve(context.Background(), c, "spec.value", src, "team-a")
	require.NoError(t, err)
	require.Equal(t, "from-secret", got)
}

func TestResolve_MissingSecret(t *testing.T) {
	c := newClient(t)
	src := apiv1.SecretValueSource{
		ValueFrom: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "missing"},
			Key:                  "k",
		},
	}
	_, err := Resolve(context.Background(), c, "spec.value", src, "ns")
	require.Error(t, err)
	var miss *ErrSecretMissing
	require.True(t, errors.As(err, &miss))
}

func TestResolve_MissingSecret_Optional(t *testing.T) {
	c := newClient(t)
	optional := true
	src := apiv1.SecretValueSource{
		ValueFrom: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "missing"},
			Key:                  "k",
			Optional:             &optional,
		},
	}
	got, err := Resolve(context.Background(), c, "spec.value", src, "ns")
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestResolve_MissingKey(t *testing.T) {
	c := newClient(t, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "ns"},
		Data:       map[string][]byte{"other": []byte("v")},
	})
	src := apiv1.SecretValueSource{
		ValueFrom: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "creds"},
			Key:                  "missing",
		},
	}
	_, err := Resolve(context.Background(), c, "spec.value", src, "ns")
	require.Error(t, err)
	var miss *ErrKeyMissing
	require.True(t, errors.As(err, &miss))
}

func TestResolve_AmbiguousAndUnconfigured(t *testing.T) {
	c := newClient(t)
	v := "x"
	_, err := Resolve(context.Background(), c, "spec.value", apiv1.SecretValueSource{
		Value: &v,
		ValueFrom: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "n"},
			Key:                  "k",
		},
	}, "ns")
	require.Error(t, err)
	var amb *ErrAmbiguous
	require.True(t, errors.As(err, &amb))

	_, err = Resolve(context.Background(), c, "spec.value", apiv1.SecretValueSource{}, "ns")
	require.Error(t, err)
	var nc *ErrNotConfigured
	require.True(t, errors.As(err, &nc))
}

func TestResolveOptional_Nil(t *testing.T) {
	c := newClient(t)
	v, ok, err := ResolveOptional(context.Background(), c, "spec.apiKey", nil, "ns")
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, v)
}

func TestResolveOptional_EmptySource(t *testing.T) {
	c := newClient(t)
	v, ok, err := ResolveOptional(context.Background(), c, "spec.apiKey", &apiv1.SecretValueSource{}, "ns")
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, v)
}
