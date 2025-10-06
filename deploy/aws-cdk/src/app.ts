#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { LlmProxyEksStack } from '../lib/llm-proxy-eks-stack';

const app = new cdk.App();

// Get configuration from context or environment
const env = {
  account: process.env.CDK_DEFAULT_ACCOUNT,
  region: process.env.CDK_DEFAULT_REGION || 'us-west-2',
};

const stackName = app.node.tryGetContext('stackName') || 'LlmProxyEks';
const clusterName = app.node.tryGetContext('clusterName') || 'llm-proxy-cluster';
const namespace = app.node.tryGetContext('namespace') || 'llm-proxy';

new LlmProxyEksStack(app, stackName, {
  env,
  clusterName,
  namespace,
  description: 'EKS cluster with LLM Proxy Helm chart deployment',
  
  // Additional configuration from context
  helmChart: {
    chartPath: app.node.tryGetContext('helmChartPath') || '../../helm/llm-proxy',
    values: app.node.tryGetContext('helmValues') || {},
  },
  
  // VPC configuration
  vpc: {
    maxAzs: 3,
    natGateways: 2,
  },
  
  // EKS configuration
  eks: {
    version: cdk.aws_eks.KubernetesVersion.V1_28,
    nodeGroups: [
      {
        name: 'compute',
        instanceTypes: [cdk.aws_ec2.InstanceType.of(cdk.aws_ec2.InstanceClass.M5, cdk.aws_ec2.InstanceSize.LARGE)],
        minSize: 2,
        maxSize: 10,
        desiredSize: 3,
      },
    ],
  },
  
  // Add-ons
  addOns: {
    awsLoadBalancerController: true,
    efsCSIDriver: true,
    ebsCSIDriver: true,
    clusterAutoscaler: true,
    metricsServer: true,
  },
  
  tags: {
    Project: 'LlmProxy',
    Environment: app.node.tryGetContext('environment') || 'dev',
    ManagedBy: 'CDK',
  },
});