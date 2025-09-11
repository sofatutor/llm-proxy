import * as cdk from 'aws-cdk-lib';
import { Template } from 'aws-cdk-lib/assertions';
import { LlmProxyEksStack } from '../lib/llm-proxy-eks-stack';

describe('LlmProxyEksStack', () => {
  test('creates EKS cluster with proper configuration', () => {
    const app = new cdk.App();
    const stack = new LlmProxyEksStack(app, 'TestStack', {
      clusterName: 'test-cluster',
      namespace: 'test-namespace',
      helmChart: {
        chartPath: '/test/path',
        values: {},
      },
    });

    const template = Template.fromStack(stack);

    // Verify EKS cluster is created
    template.hasResourceProperties('AWS::EKS::Cluster', {
      Name: 'test-cluster',
    });

    // Verify VPC is created
    template.hasResourceProperties('AWS::EC2::VPC', {
      CidrBlock: '10.0.0.0/16',
    });

    // Verify IAM roles are created
    template.hasResourceProperties('AWS::IAM::Role', {
      AssumeRolePolicyDocument: {
        Statement: [
          {
            Effect: 'Allow',
            Principal: {
              Service: 'eks.amazonaws.com',
            },
            Action: 'sts:AssumeRole',
          },
        ],
      },
    });
  });

  test('creates proper security groups', () => {
    const app = new cdk.App();
    const stack = new LlmProxyEksStack(app, 'TestStack', {
      clusterName: 'test-cluster',
      namespace: 'test-namespace',
      helmChart: {
        chartPath: '/test/path',
        values: {},
      },
    });

    const template = Template.fromStack(stack);

    // Verify security groups are created
    template.resourceCountIs('AWS::EC2::SecurityGroup', 2); // Cluster + Control plane
  });

  test('creates node groups with correct configuration', () => {
    const app = new cdk.App();
    const stack = new LlmProxyEksStack(app, 'TestStack', {
      clusterName: 'test-cluster',
      namespace: 'test-namespace',
      helmChart: {
        chartPath: '/test/path',
        values: {},
      },
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
    });

    const template = Template.fromStack(stack);

    // Verify node group is created
    template.hasResourceProperties('AWS::EKS::Nodegroup', {
      NodegroupName: 'compute',
      ScalingConfig: {
        MinSize: 2,
        MaxSize: 10,
        DesiredSize: 3,
      },
    });
  });

  test('creates secrets and IRSA correctly', () => {
    const app = new cdk.App();
    const stack = new LlmProxyEksStack(app, 'TestStack', {
      clusterName: 'test-cluster',
      namespace: 'test-namespace',
      helmChart: {
        chartPath: '/test/path',
        values: {},
      },
    });

    const template = Template.fromStack(stack);

    // Verify Secrets Manager secret is created
    template.hasResourceProperties('AWS::SecretsManager::Secret', {
      Name: 'llm-proxy/management-token',
    });

    // Verify service account IAM role is created
    template.hasResourceProperties('AWS::IAM::Role', {
      AssumeRolePolicyDocument: {
        Statement: [
          {
            Effect: 'Allow',
            Condition: {
              StringEquals: {
                'aws:RequestedRegion': cdk.Aws.REGION,
              },
            },
          },
        ],
      },
    });
  });

  test('outputs essential information', () => {
    const app = new cdk.App();
    const stack = new LlmProxyEksStack(app, 'TestStack', {
      clusterName: 'test-cluster',
      namespace: 'test-namespace',
      helmChart: {
        chartPath: '/test/path',
        values: {},
      },
    });

    const template = Template.fromStack(stack);

    // Verify outputs are created
    template.hasOutput('ClusterName', {
      Description: 'EKS Cluster Name',
    });

    template.hasOutput('ClusterEndpoint', {
      Description: 'EKS Cluster Endpoint',
    });

    template.hasOutput('KubectlCommand', {
      Description: 'Command to configure kubectl',
    });

    template.hasOutput('LlmProxyNamespace', {
      Description: 'LLM Proxy Kubernetes Namespace',
    });
  });
});