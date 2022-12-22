local c = import 'common.libsonnet';

local query = importstr '../appuio_cloud_loadbalancer.promql';

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
  }, '1x120'),

  pvCapacity: c.series('kube_service_spec_type', commonLabels {
    type: 'LoadBalancer',
    namespace: 'testproject',
  }, '1x120'),
};

local baseCalculatedLabels = {
  category: 'c-appuio-cloudscale-lpg-2:testproject',
  cluster_id: 'c-appuio-cloudscale-lpg-2',
  namespace: 'testproject',
  product: 'appuio_cloud_loadbalancer:c-appuio-cloudscale-lpg-2:cherry-pickers-inc:testproject',
  tenant_id: 'cherry-pickers-inc',
};

{
  tests: [
    c.test('minimal PVC',
           baseSeries,
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: 60,
           }),

    c.test('unrelated kube_namespace_labels changes do not throw errors - there is an overlap since series go stale only after a few missed scrapes',
           baseSeries {
             testprojectNamespaceOrgLabelUpdated: self.testprojectNamespaceOrgLabel {
               _labels+:: {
                 custom_appuio_io_myid: '672004be-a86b-44e0-b446-1255a1f8b340',
               },
               values: '_x30 1x30 _x60',
             },
           },
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: 60,
           }),

    c.test('organization changes do not throw many-to-many errors - there is an overlap since series go stale only after a few missed scrapes',
           baseSeries {
             testprojectNamespaceOrgLabel+: {
               // We cheat here and use an impossible value.
               // Since we use min() and bottomk() in the query this priotizes this series less than the other.
               // It's ugly but it prevents flaky tests since otherwise one of the series gets picked randomly.
               values: '2x120',
             },
             testprojectNamespaceOrgLabelUpdated: self.testprojectNamespaceOrgLabel {
               _labels+:: {
                 label_appuio_io_organization: 'carrot-pickers-inc',
               },
               values: '_x60 1x60',
             },
           },
           query,
           [
             {
               labels: c.formatLabels(baseCalculatedLabels),
               // 1 service * 29 * 2 because of the cheat above.
               value: 29 * 2,
             },
             {
               labels: c.formatLabels(baseCalculatedLabels {
                 tenant_id: 'carrot-pickers-inc',
                 product: 'appuio_cloud_loadbalancer:c-appuio-cloudscale-lpg-2:carrot-pickers-inc:testproject',
               }),
               value: 31,
             },
           ]),
  ],
}
