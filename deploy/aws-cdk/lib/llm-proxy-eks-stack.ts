import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as eks from 'aws-cdk-lib/aws-eks';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
import { Construct } from 'constructs';

export interface LlmProxyEksStackProps extends cdk.StackProps {
  clusterName: string;
  namespace: string;
  helmChart: {
    chartPath: string;
    values: Record<string, any>;
  };
  vpc?: {
    maxAzs: number;
    natGateways: number;
  };
  eks?: {
    version: eks.KubernetesVersion;
    nodeGroups: Array<{
      name: string;
      instanceTypes: ec2.InstanceType[];
      minSize: number;
      maxSize: number;
      desiredSize: number;
    }>;
  };
  addOns?: {
    awsLoadBalancerController?: boolean;
    efsCSIDriver?: boolean;
    ebsCSIDriver?: boolean;
    clusterAutoscaler?: boolean;
    metricsServer?: boolean;
  };
}

export class LlmProxyEksStack extends cdk.Stack {
  public readonly cluster: eks.Cluster;
  public readonly namespace: eks.KubernetesNamespace;

  constructor(scope: Construct, id: string, props: LlmProxyEksStackProps) {
    super(scope, id, props);

    // Create VPC
    const vpc = new ec2.Vpc(this, 'LlmProxyVpc', {
      maxAzs: props.vpc?.maxAzs || 3,
      natGateways: props.vpc?.natGateways || 2,
      subnetConfiguration: [
        {
          cidrMask: 24,
          name: 'public',
          subnetType: ec2.SubnetType.PUBLIC,
        },
        {
          cidrMask: 24,
          name: 'private',
          subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS,
        },
      ],
    });

    // Create IAM role for EKS cluster
    const clusterRole = new iam.Role(this, 'LlmProxyClusterRole', {
      assumedBy: new iam.ServicePrincipal('eks.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName('AmazonEKSClusterPolicy'),
      ],
    });

    // Create IAM role for node groups
    const nodeGroupRole = new iam.Role(this, 'LlmProxyNodeGroupRole', {
      assumedBy: new iam.ServicePrincipal('ec2.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName('AmazonEKSWorkerNodePolicy'),
        iam.ManagedPolicy.fromAwsManagedPolicyName('AmazonEKS_CNI_Policy'),
        iam.ManagedPolicy.fromAwsManagedPolicyName('AmazonEC2ContainerRegistryReadOnly'),
      ],
    });

    // Create EKS cluster
    this.cluster = new eks.Cluster(this, 'LlmProxyCluster', {
      clusterName: props.clusterName,
      version: props.eks?.version || eks.KubernetesVersion.V1_28,
      vpc,
      role: clusterRole,
      defaultCapacity: 0, // We'll add managed node groups separately
      endpointAccess: eks.EndpointAccess.PUBLIC_AND_PRIVATE,
      outputClusterName: true,
      outputConfigCommand: true,
      outputMastersRoleArn: true,
    });

    // Add managed node groups
    props.eks?.nodeGroups?.forEach((nodeGroupConfig, index) => {
      this.cluster.addNodegroupCapacity(`NodeGroup${index}`, {
        nodegroupName: nodeGroupConfig.name,
        instanceTypes: nodeGroupConfig.instanceTypes,
        minSize: nodeGroupConfig.minSize,
        maxSize: nodeGroupConfig.maxSize,
        desiredSize: nodeGroupConfig.desiredSize,
        nodeRole: nodeGroupRole,
        subnets: { subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS },
        amiType: eks.NodegroupAmiType.AL2_X86_64,
        capacityType: eks.CapacityType.ON_DEMAND,
        diskSize: 50,
        tags: {
          'kubernetes.io/cluster-autoscaler/enabled': 'true',
          [`kubernetes.io/cluster-autoscaler/${props.clusterName}`]: 'owned',
        },
      });
    });

    // Install add-ons
    if (props.addOns?.awsLoadBalancerController) {
      this.addAwsLoadBalancerController();
    }

    if (props.addOns?.efsCSIDriver) {
      this.addEfsCSIDriver();
    }

    if (props.addOns?.ebsCSIDriver) {
      this.addEbsCSIDriver();
    }

    if (props.addOns?.clusterAutoscaler) {
      this.addClusterAutoscaler(props.clusterName);
    }

    if (props.addOns?.metricsServer) {
      this.addMetricsServer();
    }

    // Create namespace for LLM Proxy
    this.namespace = this.cluster.addManifest('LlmProxyNamespace', {
      apiVersion: 'v1',
      kind: 'Namespace',
      metadata: {
        name: props.namespace,
        labels: {
          'app.kubernetes.io/name': 'llm-proxy',
          'app.kubernetes.io/managed-by': 'cdk',
        },
      },
    });

    // Create secrets for LLM Proxy
    this.createSecrets(props.namespace);

    // Create IRSA for LLM Proxy
    const serviceAccount = this.createServiceAccount(props.namespace);

    // Deploy Helm chart
    this.deployHelmChart(props);

    // Output important information
    new cdk.CfnOutput(this, 'ClusterName', {
      value: this.cluster.clusterName,
      description: 'EKS Cluster Name',
    });

    new cdk.CfnOutput(this, 'ClusterEndpoint', {
      value: this.cluster.clusterEndpoint,
      description: 'EKS Cluster Endpoint',
    });

    new cdk.CfnOutput(this, 'KubectlCommand', {
      value: `aws eks update-kubeconfig --region ${this.region} --name ${this.cluster.clusterName}`,
      description: 'Command to configure kubectl',
    });

    new cdk.CfnOutput(this, 'LlmProxyNamespace', {
      value: props.namespace,
      description: 'LLM Proxy Kubernetes Namespace',
    });
  }

  private addAwsLoadBalancerController(): void {
    // Add AWS Load Balancer Controller
    this.cluster.addHelmChart('AwsLoadBalancerController', {
      chart: 'aws-load-balancer-controller',
      repository: 'https://aws.github.io/eks-charts',
      namespace: 'kube-system',
      values: {
        clusterName: this.cluster.clusterName,
        serviceAccount: {
          create: false,
          name: 'aws-load-balancer-controller',
        },
      },
    });

    // Create IRSA for AWS Load Balancer Controller
    const albServiceAccount = this.cluster.addServiceAccount('aws-load-balancer-controller', {
      name: 'aws-load-balancer-controller',
      namespace: 'kube-system',
    });

    albServiceAccount.role.addManagedPolicy(
      iam.ManagedPolicy.fromAwsManagedPolicyName('ElasticLoadBalancingFullAccess')
    );
  }

  private addEfsCSIDriver(): void {
    this.cluster.addHelmChart('EfsCSIDriver', {
      chart: 'aws-efs-csi-driver',
      repository: 'https://kubernetes-sigs.github.io/aws-efs-csi-driver',
      namespace: 'kube-system',
    });
  }

  private addEbsCSIDriver(): void {
    this.cluster.addHelmChart('EbsCSIDriver', {
      chart: 'aws-ebs-csi-driver',
      repository: 'https://kubernetes-sigs.github.io/aws-ebs-csi-driver',
      namespace: 'kube-system',
    });
  }

  private addClusterAutoscaler(clusterName: string): void {
    this.cluster.addHelmChart('ClusterAutoscaler', {
      chart: 'cluster-autoscaler',
      repository: 'https://kubernetes.github.io/autoscaler',
      namespace: 'kube-system',
      values: {
        autoDiscovery: {
          clusterName: clusterName,
        },
        awsRegion: this.region,
      },
    });
  }

  private addMetricsServer(): void {
    this.cluster.addHelmChart('MetricsServer', {
      chart: 'metrics-server',
      repository: 'https://kubernetes-sigs.github.io/metrics-server',
      namespace: 'kube-system',
    });
  }

  private createSecrets(namespace: string): void {
    // Create AWS Secrets Manager secret for management token
    const managementTokenSecret = new secretsmanager.Secret(this, 'LlmProxyManagementToken', {
      secretName: `llm-proxy/management-token`,
      description: 'LLM Proxy Management Token',
      generateSecretString: {
        secretStringTemplate: JSON.stringify({}),
        generateStringKey: 'token',
        excludeCharacters: '"@/\\\'',
        passwordLength: 32,
      },
    });

    // Create External Secret for management token
    this.cluster.addManifest('LlmProxyManagementTokenExternalSecret', {
      apiVersion: 'external-secrets.io/v1beta1',
      kind: 'ExternalSecret',
      metadata: {
        name: 'llm-proxy-management-token',
        namespace: namespace,
      },
      spec: {
        refreshInterval: '1h',
        secretStoreRef: {
          name: 'aws-secrets-manager',
          kind: 'SecretStore',
        },
        target: {
          name: 'llm-proxy-secrets',
          creationPolicy: 'Owner',
        },
        data: [
          {
            secretKey: 'management-token',
            remoteRef: {
              key: managementTokenSecret.secretName,
              property: 'token',
            },
          },
        ],
      },
    });

    // Create SecretStore for AWS Secrets Manager
    this.cluster.addManifest('AwsSecretsManagerStore', {
      apiVersion: 'external-secrets.io/v1beta1',
      kind: 'SecretStore',
      metadata: {
        name: 'aws-secrets-manager',
        namespace: namespace,
      },
      spec: {
        provider: {
          aws: {
            service: 'SecretsManager',
            region: this.region,
            auth: {
              jwt: {
                serviceAccountRef: {
                  name: 'llm-proxy',
                },
              },
            },
          },
        },
      },
    });
  }

  private createServiceAccount(namespace: string): eks.ServiceAccount {
    // Create IRSA for LLM Proxy with necessary permissions
    const serviceAccount = this.cluster.addServiceAccount('llm-proxy', {
      name: 'llm-proxy',
      namespace: namespace,
    });

    // Add permissions for Secrets Manager
    serviceAccount.role.addToPolicy(new iam.PolicyStatement({
      effect: iam.Effect.ALLOW,
      actions: [
        'secretsmanager:GetSecretValue',
        'secretsmanager:DescribeSecret',
      ],
      resources: [`arn:aws:secretsmanager:${this.region}:${this.account}:secret:llm-proxy/*`],
    }));

    // Add permissions for CloudWatch (if needed for logging)
    serviceAccount.role.addToPolicy(new iam.PolicyStatement({
      effect: iam.Effect.ALLOW,
      actions: [
        'logs:CreateLogGroup',
        'logs:CreateLogStream',
        'logs:PutLogEvents',
        'logs:DescribeLogStreams',
      ],
      resources: [`arn:aws:logs:${this.region}:${this.account}:log-group:/aws/llm-proxy/*`],
    }));

    return serviceAccount;
  }

  private deployHelmChart(props: LlmProxyEksStackProps): void {
    // Deploy LLM Proxy Helm chart
    const helmChart = this.cluster.addHelmChart('LlmProxy', {
      chart: 'llm-proxy',
      chartAsset: cdk.aws_s3_assets.Asset.fromAsset(props.helmChart.chartPath),
      namespace: props.namespace,
      timeout: cdk.Duration.minutes(10),
      wait: true,
      values: {
        // Default values
        image: {
          repository: 'ghcr.io/sofatutor/llm-proxy',
          tag: 'latest',
        },
        serviceAccount: {
          create: false,
          name: 'llm-proxy',
        },
        secrets: {
          create: false,
          external: true,
          externalSecrets: {
            managementToken: 'llm-proxy-secrets',
          },
        },
        ingress: {
          enabled: true,
          className: 'alb',
          annotations: {
            'kubernetes.io/ingress.class': 'alb',
            'alb.ingress.kubernetes.io/scheme': 'internet-facing',
            'alb.ingress.kubernetes.io/target-type': 'ip',
            'alb.ingress.kubernetes.io/healthcheck-path': '/health',
            'alb.ingress.kubernetes.io/ssl-redirect': '443',
          },
        },
        autoscaling: {
          enabled: true,
          minReplicas: 2,
          maxReplicas: 10,
        },
        podDisruptionBudget: {
          enabled: true,
          minAvailable: 1,
        },
        serviceMonitor: {
          enabled: true,
        },
        networkPolicy: {
          enabled: true,
        },
        // Merge with custom values
        ...props.helmChart.values,
      },
    });

    // Add dependency on namespace and service account
    helmChart.node.addDependency(this.namespace);
  }
}