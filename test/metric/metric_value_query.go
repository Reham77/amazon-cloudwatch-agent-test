// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

//go:build linux && integration
// +build linux,integration

package metric

import (
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/aws/amazon-cloudwatch-agent-test/test"
)

var metricValueFetchers = []MetricValueFetcher{
	&CPUMetricValueFetcher{},
	&MemMetricValueFetcher{},
	&ProcStatMetricValueFetcher{},
	&DiskIOMetricValueFetcher{},
}

func GetMetricFetcher(metricName string) (MetricValueFetcher, error) {
	for _, fetcher := range metricValueFetchers {
		if fetcher.isApplicable(metricName) {
			return fetcher, nil
		}
	}
	err := fmt.Errorf("No metric fetcher for metricName %v", metricName)
	log.Printf("%s", err)
	return nil, err
}

type MetricValueFetcher interface {
	// Fetch and fetch uses GetMetricData to return the metrics value being submitted
	//in the last 10 minutes with the given metric name, namespace, and dimensions.
	Fetch(namespace, metricName string, stat Statistics) (MetricValues, error)
	fetch(namespace, metricName string, metricSpecificDimensions []types.Dimension, stat Statistics) (MetricValues, error)

	// isApplicable checks whether the given metric is supported within the plugin
	//(e.g cpu_time_active is supported in CPU plugin while mem_active does not)
	isApplicable(metricName string) bool

	// getMetricSpecificDimensions returns the dimensions that needs to be scraped by each plugin
	getMetricSpecificDimensions() []types.Dimension

	// getPluginSupportedMetric returns the supported metrics for each plugin
	// https://github.com/aws/amazon-cloudwatch-agent/blob/6451e8b913bcf9892f2cead08e335c913c690e6d/translator/translate/metrics/config/registered_metrics.go
	getPluginSupportedMetric() map[string]struct{}
}

type baseMetricValueFetcher struct{}

func (f *baseMetricValueFetcher) fetch(namespace, metricName string, metricSpecificDimensions []types.Dimension, stat Statistics) (MetricValues, error) {
	ec2InstanceId := test.GetInstanceId()
	instanceIdDimension := types.Dimension{
		Name:  aws.String("InstanceId"),
		Value: aws.String(ec2InstanceId),
	}
	dimensions := append(metricSpecificDimensions, instanceIdDimension)
	metricToFetch := types.Metric{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		Dimensions: dimensions,
	}

	metricQueryPeriod := int32(60)
	metricDataQueries := []types.MetricDataQuery{
		{
			MetricStat: &types.MetricStat{
				Metric: &metricToFetch,
				Period: &metricQueryPeriod,
				Stat:   aws.String(string(stat)),
			},
			Id: aws.String(metricName),
		},
	}

	endTime := time.Now()
	startTime := subtractMinutes(endTime, 10)
	getMetricDataInput := cloudwatch.GetMetricDataInput{
		StartTime:         &startTime,
		EndTime:           &endTime,
		MetricDataQueries: metricDataQueries,
	}

	log.Printf("Metric data input is : %s", fmt.Sprint(getMetricDataInput))

	cwmClient, clientContext, err := test.GetCloudWatchMetricsClient()
	if err != nil {
		return nil, fmt.Errorf("Error occurred while creating CloudWatch client: %v", err.Error())
	}

	output, err := cwmClient.GetMetricData(*clientContext, &getMetricDataInput)
	if err != nil {
		return nil, fmt.Errorf("Error getting metric data %v", err)
	}

	result := output.MetricDataResults[0].Values
	log.Printf("Metric Value is : %s", fmt.Sprint(result))

	return result, nil
}

func subtractMinutes(fromTime time.Time, minutes int) time.Time {
	tenMinutes := time.Duration(-1*minutes) * time.Minute
	return fromTime.Add(tenMinutes)
}
