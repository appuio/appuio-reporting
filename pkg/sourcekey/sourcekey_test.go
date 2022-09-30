package sourcekey_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/appuio/appuio-cloud-reporting/pkg/sourcekey"
)

func TestParseInvalidKey(t *testing.T) {
	_, err := sourcekey.Parse("appuio_cloud_storage:c-appuio-cloudscale-lpg-2")
	require.Error(t, err)
}

func TestParseWithClass(t *testing.T) {
	k, err := sourcekey.Parse("appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234:ssd")
	require.NoError(t, err)
	require.Equal(t, k, sourcekey.SourceKey{
		Query:     "appuio_cloud_storage",
		Zone:      "c-appuio-cloudscale-lpg-2",
		Tenant:    "acme-corp",
		Namespace: "sparkling-sound-1234",
		Class:     "ssd",
	})
}

func TestParseWithoutClass(t *testing.T) {
	k, err := sourcekey.Parse("appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234")
	require.NoError(t, err)
	require.Equal(t, k, sourcekey.SourceKey{
		Query:     "appuio_cloud_storage",
		Zone:      "c-appuio-cloudscale-lpg-2",
		Tenant:    "acme-corp",
		Namespace: "sparkling-sound-1234",
	})
}

func TestParseWithEmptyClass(t *testing.T) {
	k, err := sourcekey.Parse("appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234:")
	require.NoError(t, err)
	require.Equal(t, k, sourcekey.SourceKey{
		Query:     "appuio_cloud_storage",
		Zone:      "c-appuio-cloudscale-lpg-2",
		Tenant:    "acme-corp",
		Namespace: "sparkling-sound-1234",
	})
}

func TestStringWithClass(t *testing.T) {
	key := sourcekey.SourceKey{
		Query:     "appuio_cloud_storage",
		Zone:      "c-appuio-cloudscale-lpg-2",
		Tenant:    "acme-corp",
		Namespace: "sparkling-sound-1234",
		Class:     "ssd",
	}
	require.Equal(t, "appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234:ssd", key.String())
}

func TestStringWithoutClass(t *testing.T) {
	key := sourcekey.SourceKey{
		Query:     "appuio_cloud_storage",
		Zone:      "c-appuio-cloudscale-lpg-2",
		Tenant:    "acme-corp",
		Namespace: "sparkling-sound-1234",
	}
	require.Equal(t, "appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234", key.String())
}

func TestGenerateSourceKeysWithoutClass(t *testing.T) {
	keys := sourcekey.SourceKey{
		Query:     "appuio_cloud_storage",
		Zone:      "c-appuio-cloudscale-lpg-2",
		Tenant:    "acme-corp",
		Namespace: "sparkling-sound-1234",
	}.LookupKeys()

	require.Equal(t, keys, []string{
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:*:sparkling-sound-1234",
		"appuio_cloud_storage:*:acme-corp:sparkling-sound-1234",
		"appuio_cloud_storage:*:*:sparkling-sound-1234",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp",
		"appuio_cloud_storage:*:acme-corp",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2",
		"appuio_cloud_storage",
	})
}

func TestGenerateSourceKeysWithClass(t *testing.T) {
	keys := sourcekey.SourceKey{
		Query:     "appuio_cloud_storage",
		Zone:      "c-appuio-cloudscale-lpg-2",
		Tenant:    "acme-corp",
		Namespace: "sparkling-sound-1234",
		Class:     "ssd",
	}.LookupKeys()

	require.Equal(t, keys, []string{
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234:ssd",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:*:ssd",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:*:sparkling-sound-1234:ssd",
		"appuio_cloud_storage:*:acme-corp:sparkling-sound-1234:ssd",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:*:*:ssd",
		"appuio_cloud_storage:*:acme-corp:*:ssd",
		"appuio_cloud_storage:*:*:sparkling-sound-1234:ssd",
		"appuio_cloud_storage:*:*:*:ssd",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:*:sparkling-sound-1234",
		"appuio_cloud_storage:*:acme-corp:sparkling-sound-1234",
		"appuio_cloud_storage:*:*:sparkling-sound-1234",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp",
		"appuio_cloud_storage:*:acme-corp",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2",
		"appuio_cloud_storage",
	})
}
