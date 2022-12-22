local c = import 'common.libsonnet';

local query = importstr '../appuio_cloud_persistent_storage.promql';

local commonLabels = {
  cluster_id: 'c-appuio-cloudscale-lpg-2',
  tenant_id: 'c-appuio-cloudscale-lpg-2',
};

// One pvc, minimal (=1 byte) request
// 10 samples
local baseSeries = {
  testprojectNamespaceOrgLabel: c.series('kube_namespace_labels', commonLabels {
    namespace: 'testproject',
    label_appuio_io_organization: 'cherry-pickers-inc',
  }, '1x10'),

  local pvcID = 'pvc-da01b12d-2e31-44da-8312-f91169256221',
  pvCapacity: c.series('kube_persistentvolume_capacity_bytes', commonLabels {
    persistentvolume: pvcID,
  }, '1x10'),
  pvInfo: c.series('kube_persistentvolume_info', commonLabels {
    persistentvolume: pvcID,
    storageclass: 'ssd',
  }, '1x10'),
  pvcRef: c.series('kube_persistentvolume_claim_ref', commonLabels {
    claim_namespace: 'testproject',
    name: 'important-database',
    persistentvolume: pvcID,
  }, '1x10'),
};

local baseCalculatedLabels = {
  category: 'c-appuio-cloudscale-lpg-2:testproject',
  cluster_id: 'c-appuio-cloudscale-lpg-2',
  namespace: 'testproject',
  product: 'appuio_cloud_persistent_storage:c-appuio-cloudscale-lpg-2:cherry-pickers-inc:testproject:ssd',
  storageclass: 'ssd',
  tenant_id: 'cherry-pickers-inc',
};

{
  tests: [
    c.test('minimal PVC',
           baseSeries,
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: 10,
           }),
    c.test('higher than 1GiB request',
           baseSeries {
             pvCapacity+: {
               values: '%sx10' % (5 * 1024 * 1024 * 1024),
             },
           },
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: 5 * 10,
           }),

    c.test('unrelated kube_persistentvolume_info changes do not throw errors - there is an overlap since series go stale only after a few missed scrapes',
           baseSeries {
             pvInfoUpdated: self.pvInfo {
               _labels+:: {
                 csi_volume_handle: '672004be-a86b-44e0-b446-1255a1f8b340',
               },
               values: '_x5 1x5',
             },
           },
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: 10,
           }),
  ],
}
