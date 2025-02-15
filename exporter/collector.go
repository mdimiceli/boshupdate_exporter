package main

import (
	"github.com/orange-cloudfoundry/boshupdate_exporter/boshupdate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"time"
)

// BoshUpdateCollector -
type BoshUpdateCollector struct {
	manager                         *boshupdate.Manager
	manifestRelease                 *prometheus.GaugeVec
	manifestBoshRelease             *prometheus.GaugeVec
	deploymentStatus                *prometheus.GaugeVec
	deploymentReleaseStatus         *prometheus.GaugeVec
	genericRelease                  *prometheus.GaugeVec
	lastScrapeTimestampMetric       prometheus.Gauge
	lastScrapeErrorMetric           prometheus.Gauge
	lastScrapeDurationSecondsMetric prometheus.Gauge
}

// NewBoshUpdateCollector -
func NewBoshUpdateCollector(namespace string, environment string, manager *boshupdate.Manager) *BoshUpdateCollector {
	manifestRelease := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "manifest_release",
			Help:        "Seconds from epoch since deployment release is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"name", "version", "owner", "repo"},
	)

	manifestBoshRelease := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "manifest_bosh_release_info",
			Help:        "Informational metric that gives the bosh release versions requests by the latest version of a manifest release, (always 0)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"manifest_name", "manifest_version", "owner", "repo", "boshrelease_name", "boshrelease_version", "boshrelease_url"},
	)

	genericRelease := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "generic_release",
			Help:        "Seconds from epoch since github release is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"name", "version", "owner", "repo"},
	)

	deploymentStatus := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "deployment_status",
			Help:        "Seconds from epoch since this deployment is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"deployment", "name", "current", "latest"},
	)

	deploymentReleaseStatus := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "deployment_bosh_release_status",
			Help:        "Seconds from epoch since this bosh release is out of date, (0 means up to date)",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
		[]string{"deployment", "manifest_name", "manifest_current", "manifest_latest", "boshrelease_name", "boshrelease_current", "boshrelease_latest"},
	)

	lastScrapeTimestampMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "last_scrape_timestamp",
			Help:        "Seconds from epoch since last scrape of metrics from boshupdate.",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
	)

	lastScrapeErrorMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "last_scrape_error",
			Help:        "Number of errors in last scrape of metrics.",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
	)

	lastScrapeDurationSecondsMetric := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Subsystem:   "",
			Name:        "last_scrape_duration",
			Help:        "Duration of the last scrape.",
			ConstLabels: prometheus.Labels{"environment": environment},
		},
	)

	return &BoshUpdateCollector{
		manager:                         manager,
		manifestRelease:                 manifestRelease,
		manifestBoshRelease:             manifestBoshRelease,
		deploymentStatus:                deploymentStatus,
		deploymentReleaseStatus:         deploymentReleaseStatus,
		genericRelease:                  genericRelease,
		lastScrapeTimestampMetric:       lastScrapeTimestampMetric,
		lastScrapeErrorMetric:           lastScrapeErrorMetric,
		lastScrapeDurationSecondsMetric: lastScrapeDurationSecondsMetric,
	}
}

// getVersion -
// fetch deployment.Versions match manifest.Name
func (c BoshUpdateCollector) getVersion(
	deployment boshupdate.BoshDeploymentData,
	releases []boshupdate.ManifestReleaseData) (*boshupdate.ManifestReleaseData, *boshupdate.Version) {

	for _, r := range releases {
		if !r.Match(deployment.ManifestName) {
			continue
		}
		for _, v := range r.Versions {
			if v.Version == deployment.Ref {
				return &r, &v
			}
		}
	}
	return nil, nil
}

func (c BoshUpdateCollector) getBoshReleaseVersion(
	manifest *boshupdate.ManifestReleaseData,
	boshRelease boshupdate.BoshRelease) *boshupdate.BoshRelease {
	for _, br := range manifest.BoshReleases {
		if br.Name == boshRelease.Name {
			return &br
		}
	}
	return nil
}

// Collect -
func (c BoshUpdateCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debugf("collecting boshupdate metrics")

	startTime := time.Now()
	c.lastScrapeErrorMetric.Set(0.0)
	c.lastScrapeTimestampMetric.Set(float64(time.Now().Unix()))

	manifests := c.manager.GetManifestReleases()
	for _, m := range manifests {
		if m.HasError {
			log.Warnf("error during analysis of manifest release '%s'", m.Name)
			c.lastScrapeErrorMetric.Add(1.0)
			continue
		}
		for _, v := range m.Versions {
			c.manifestRelease.
				WithLabelValues(m.Name, v.Version, m.Owner, m.Repo).
				Set(float64(v.ExpiredSince))
		}
		for _, r := range m.BoshReleases {
			c.manifestBoshRelease.
				WithLabelValues(m.Name, m.LatestVersion.Version, m.Owner, m.Repo, r.Name, r.Version, r.URL).
				Set(float64(0))
		}
	}

	generics := c.manager.GetGenericReleases()
	for _, r := range generics {
		if r.HasError {
			log.Warnf("error during analysis of github release '%s'", r.Name)
			c.lastScrapeErrorMetric.Add(1.0)
			continue
		}
		for _, v := range r.Versions {
			c.genericRelease.
				WithLabelValues(r.Name, v.Version, r.Owner, r.Repo).
				Set(float64(v.ExpiredSince))
		}
	}

	deployments, err := c.manager.GetBoshDeployments()
	if err != nil {
		log.Errorf("unable to get bosh deployments: %s", err)
		c.lastScrapeErrorMetric.Add(1.0)
	}

	for _, d := range deployments {
		if d.HasError {
			c.lastScrapeErrorMetric.Add(1.0)
			log.Warnf("error during analysis of deployment '%s'", d.Deployment)
			continue
		}

		manifest, version := c.getVersion(d, manifests)
		if manifest == nil || version == nil {
			c.deploymentStatus.
				WithLabelValues(d.Deployment, d.ManifestName, d.Ref, "not-found").
				Set(0)
		} else {
			c.deploymentStatus.
				WithLabelValues(d.Deployment, manifest.Name, version.Version, manifest.LatestVersion.Version).
				Set(float64(version.ExpiredSince))
			for _, br := range d.BoshReleases {
				latestBr := c.getBoshReleaseVersion(manifest, br)
				if latestBr == nil {
					c.deploymentReleaseStatus.
						WithLabelValues(d.Deployment, manifest.Name, version.Version, manifest.LatestVersion.Version, br.Name, br.Version, "not-found").
						Set(0)
				} else {
					value := version.ExpiredSince
					if br.Version == latestBr.Version {
						value = 0
					}
					c.deploymentReleaseStatus.
						WithLabelValues(d.Deployment, manifest.Name, version.Version, manifest.LatestVersion.Version, br.Name, br.Version, latestBr.Version).
						Set(float64(value))
				}
			}
		}
	}

	duration := time.Since(startTime).Seconds()

	c.lastScrapeDurationSecondsMetric.Set(duration)
	c.manifestRelease.Collect(ch)
	c.manifestBoshRelease.Collect(ch)
	c.deploymentStatus.Collect(ch)
	c.deploymentReleaseStatus.Collect(ch)
	c.genericRelease.Collect(ch)
	c.lastScrapeTimestampMetric.Collect(ch)
	c.lastScrapeErrorMetric.Collect(ch)
	c.lastScrapeDurationSecondsMetric.Collect(ch)
}

// Describe -
func (c BoshUpdateCollector) Describe(ch chan<- *prometheus.Desc) {
	c.manifestRelease.Describe(ch)
	c.manifestBoshRelease.Describe(ch)
	c.deploymentStatus.Describe(ch)
	c.deploymentReleaseStatus.Describe(ch)
	c.genericRelease.Describe(ch)
	c.lastScrapeTimestampMetric.Describe(ch)
	c.lastScrapeErrorMetric.Describe(ch)
	c.lastScrapeDurationSecondsMetric.Describe(ch)
}

// Local Variables:
// ispell-local-dictionary: "american"
// End:
