local c = import 'common.libsonnet';

local query = importstr '../appuio_managed_openshift_clusters_legacy.promql';

local commonLabels = {
  cluster_id: 'c-managed-openshift',
};

local infoLabels = commonLabels {
  tenant_id: 't-managed-openshift',
  vshn_service_level: 'standard',
  cloud_provider: 'cloudscale',
};

local baseSeries = {
  appuioInfoLabel: c.series('appuio_managed_info', infoLabels, '1x120'),
  appuioInfoLabel2: c.series('appuio_managed_info', infoLabels {
    vshn_service_level: 'best_effort',
  }, '1x120'),
};

local baseCalculatedLabels = infoLabels {
  class: super.vshn_service_level,
  category: super.tenant_id + ':' + super.cluster_id,
};

{
  tests: [
    c.test(
      'one cluster',
      baseSeries,
      query,
      [
        {
          labels: c.formatLabels(baseCalculatedLabels {
            product: 'appuio_managed_openshift_clusters:cloudscale:t-managed-openshift:c-managed-openshift:standard',
          }),
          value: 1,
        },
      ]
    ),

  ],
}
