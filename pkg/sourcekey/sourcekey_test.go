package sourcekey

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseInvalidKey(t *testing.T) {
	_, err := Parse("appuio_cloud_storage:c-appuio-cloudscale-lpg-2")
	require.Error(t, err)
}

func TestParseWithclass(t *testing.T) {
	k, err := Parse("appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234:ssd")
	require.NoError(t, err)
	require.Equal(t, SourceKey{
		parts: []string{"appuio_cloud_storage", "c-appuio-cloudscale-lpg-2", "acme-corp", "sparkling-sound-1234", "ssd"},
	}, k)
}

func TestParseWithoutclass(t *testing.T) {
	k, err := Parse("appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234")
	require.NoError(t, err)
	require.Equal(t, SourceKey{
		parts: []string{"appuio_cloud_storage", "c-appuio-cloudscale-lpg-2", "acme-corp", "sparkling-sound-1234"},
	}, k)
}

func TestParseWithEmptyclass(t *testing.T) {
	k, err := Parse("appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234:")
	require.NoError(t, err)
	require.Equal(t, SourceKey{
		parts: []string{"appuio_cloud_storage", "c-appuio-cloudscale-lpg-2", "acme-corp", "sparkling-sound-1234"},
	}, k)
}

func TestStringWithclass(t *testing.T) {
	key := SourceKey{
		parts: []string{"appuio_cloud_storage", "c-appuio-cloudscale-lpg-2", "acme-corp", "sparkling-sound-1234", "ssd"},
	}
	require.Equal(t, "appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234:ssd", key.String())
}

func TestStringWithoutclass(t *testing.T) {
	key := SourceKey{
		parts: []string{"appuio_cloud_storage", "c-appuio-cloudscale-lpg-2", "acme-corp", "sparkling-sound-1234"},
	}
	require.Equal(t, "appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234", key.String())
}

func TestGenerateSourceKeysWithoutclass(t *testing.T) {
	keys := SourceKey{
		parts: []string{"appuio_cloud_storage", "c-appuio-cloudscale-lpg-2", "acme-corp", "sparkling-sound-1234"},
	}.LookupKeys()

	require.Equal(t, []string{
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:*:sparkling-sound-1234",
		"appuio_cloud_storage:*:acme-corp:sparkling-sound-1234",
		"appuio_cloud_storage:*:*:sparkling-sound-1234",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp",
		"appuio_cloud_storage:*:acme-corp",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2",
		"appuio_cloud_storage",
	}, keys)
}

func TestGenerateSourceKeysWithclass(t *testing.T) {
	keys := SourceKey{
		parts: []string{"appuio_cloud_storage", "c-appuio-cloudscale-lpg-2", "acme-corp", "sparkling-sound-1234", "ssd"},
	}.LookupKeys()

	require.Equal(t, []string{
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
	}, keys)
}

func TestGenerateSourceKeysWithSixElements(t *testing.T) {
	keys := SourceKey{
		parts: []string{"appuio_cloud_storage", "c-appuio-cloudscale-lpg-2", "acme-corp", "sparkling-sound-1234", "ssd", "exoscale"},
	}.LookupKeys()

	require.Equal(t, []string{
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234:ssd:exoscale",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:sparkling-sound-1234:*:exoscale",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:*:ssd:exoscale",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:*:sparkling-sound-1234:ssd:exoscale",
		"appuio_cloud_storage:*:acme-corp:sparkling-sound-1234:ssd:exoscale",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:acme-corp:*:*:exoscale",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:*:sparkling-sound-1234:*:exoscale",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:*:*:ssd:exoscale",
		"appuio_cloud_storage:*:acme-corp:sparkling-sound-1234:*:exoscale",
		"appuio_cloud_storage:*:acme-corp:*:ssd:exoscale",
		"appuio_cloud_storage:*:*:sparkling-sound-1234:ssd:exoscale",
		"appuio_cloud_storage:c-appuio-cloudscale-lpg-2:*:*:*:exoscale",
		"appuio_cloud_storage:*:acme-corp:*:*:exoscale",
		"appuio_cloud_storage:*:*:sparkling-sound-1234:*:exoscale",
		"appuio_cloud_storage:*:*:*:ssd:exoscale",
		"appuio_cloud_storage:*:*:*:*:exoscale",
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
	}, keys)
}
