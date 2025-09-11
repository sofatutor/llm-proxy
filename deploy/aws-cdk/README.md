# AWS CDK Deployment for EKS

This directory contains AWS CDK (Cloud Development Kit) code for deploying LLM Proxy to Amazon EKS (Elastic Kubernetes Service). The CDK stack provides a complete infrastructure-as-code solution for production-ready deployments.

## Features

- **Complete EKS cluster setup** with managed node groups
- **VPC configuration** with public/private subnets
- **IAM roles and service accounts** with least-privilege access
- **AWS Load Balancer Controller** for ingress management
- **EBS and EFS CSI drivers** for persistent storage
- **Cluster autoscaler** for automatic node scaling
- **External Secrets Operator** integration with AWS Secrets Manager
- **Helm chart deployment** with production-ready configuration
- **Monitoring and observability** setup

## Prerequisites

### Tools

- **Node.js** (v18+)
- **AWS CLI** configured with appropriate credentials
- **AWS CDK** (v2.80+)
- **kubectl** (for cluster management)
- **Helm** (v3.8+)

### AWS Permissions

Your AWS credentials need the following permissions:
- EKS cluster creation and management
- VPC and networking resources
- IAM role creation and management
- Secrets Manager access
- EC2 instance management

## Quick Start

### 1. Install Dependencies

```bash
cd deploy/aws-cdk
npm install
```

### 2. Configure AWS CLI

```bash
aws configure
# or use AWS SSO
aws sso login --profile your-profile
```

### 3. Bootstrap CDK (First Time Only)

```bash
# Bootstrap CDK in your AWS account/region
npx cdk bootstrap

# If using a specific profile
npx cdk bootstrap --profile your-profile
```

### 4. Deploy Development Environment

```bash
# Deploy with development configuration
npx cdk deploy \
  --context @cdk-dev.json \
  --require-approval never

# Or specify individual parameters
npx cdk deploy \
  --context stackName=LlmProxyEksDev \
  --context clusterName=llm-proxy-dev \
  --context environment=dev
```

### 5. Configure kubectl

```bash
# Update kubeconfig
aws eks update-kubeconfig --region us-west-2 --name llm-proxy-dev

# Verify connectivity
kubectl get nodes
kubectl get pods -n llm-proxy
```

## Configuration

### Context Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `stackName` | CloudFormation stack name | `LlmProxyEks` |
| `clusterName` | EKS cluster name | `llm-proxy-cluster` |
| `namespace` | Kubernetes namespace | `llm-proxy` |
| `environment` | Environment tag | `dev` |
| `helmChartPath` | Path to Helm chart | `../../helm/llm-proxy` |
| `helmValues` | Helm chart values override | `{}` |

### Example Deployments

#### Development Environment

```bash
npx cdk deploy --context @examples/cdk-dev.json
```

#### Production Environment

```bash
npx cdk deploy --context @examples/cdk-prod.json
```

#### Custom Configuration

```bash
npx cdk deploy \
  --context stackName=MyLlmProxy \
  --context clusterName=my-cluster \
  --context environment=staging \
  --context 'helmValues={"autoscaling":{"enabled":true}}'
```

## Infrastructure Components

### EKS Cluster

- **Kubernetes Version**: 1.28 (configurable)
- **Endpoint Access**: Public and private
- **Node Groups**: Managed node groups with auto-scaling
- **Add-ons**: AWS Load Balancer Controller, EBS CSI, EFS CSI, Cluster Autoscaler

### Networking

- **VPC**: Multi-AZ with public and private subnets
- **Security Groups**: Least-privilege access rules
- **Network Policies**: Optional pod-to-pod communication control

### Storage

- **EBS CSI Driver**: For persistent volume claims
- **EFS CSI Driver**: For shared storage (if needed)
- **Storage Classes**: gp3 for high-performance storage

### Security

- **IAM Roles for Service Accounts (IRSA)**: Fine-grained permissions
- **AWS Secrets Manager**: Secure secrets storage
- **External Secrets Operator**: Kubernetes secrets sync
- **Pod Security Standards**: Security contexts and policies

### Monitoring

- **Metrics Server**: Resource metrics collection
- **CloudWatch Integration**: Logs and metrics forwarding
- **Prometheus Integration**: ServiceMonitor and PodMonitor resources

## Secrets Management

### AWS Secrets Manager

The CDK stack automatically creates and manages secrets:

```bash
# View created secrets
aws secretsmanager list-secrets --query 'SecretList[?contains(Name, `llm-proxy`)]'

# Get management token
aws secretsmanager get-secret-value \
  --secret-id llm-proxy/management-token \
  --query SecretString --output text
```

### External Secrets Operator

The stack deploys External Secrets Operator to sync AWS secrets to Kubernetes:

```yaml
# Example ExternalSecret (automatically created)
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: llm-proxy-management-token
  namespace: llm-proxy
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: llm-proxy-secrets
  data:
  - secretKey: management-token
    remoteRef:
      key: llm-proxy/management-token
      property: token
```

## Custom Helm Values

### Override via Context

```bash
npx cdk deploy --context 'helmValues={
  "autoscaling": {"enabled": true, "minReplicas": 3},
  "ingress": {"enabled": true, "hosts": [{"host": "my-domain.com"}]}
}'
```

### Configuration File

Create a custom configuration file:

```json
{
  "stackName": "MyLlmProxy",
  "helmValues": {
    "image": {"tag": "v1.2.3"},
    "resources": {
      "limits": {"cpu": "2000m", "memory": "2Gi"}
    },
    "redis": {
      "enabled": false,
      "external": {"host": "my-redis.elasticache.aws"}
    }
  }
}
```

## Operations

### Cluster Management

```bash
# View cluster info
kubectl cluster-info

# Scale node groups
aws eks update-nodegroup-config \
  --cluster-name llm-proxy-cluster \
  --nodegroup-name compute \
  --scaling-config minSize=2,maxSize=20,desiredSize=5

# Update cluster version
aws eks update-cluster-version \
  --name llm-proxy-cluster \
  --kubernetes-version 1.29
```

### Application Management

```bash
# Check deployment status
kubectl get deployment -n llm-proxy

# View logs
kubectl logs -n llm-proxy deployment/llm-proxy -f

# Port forward for testing
kubectl port-forward -n llm-proxy svc/llm-proxy 8080:8080

# Run Helm tests
helm test llm-proxy -n llm-proxy
```

### Monitoring

```bash
# View metrics
kubectl top pods -n llm-proxy
kubectl top nodes

# Check autoscaler status
kubectl logs -n kube-system deployment/cluster-autoscaler -f

# View ingress
kubectl get ingress -n llm-proxy
kubectl describe ingress -n llm-proxy
```

## Upgrade and Maintenance

### CDK Stack Updates

```bash
# View changes
npx cdk diff

# Deploy updates
npx cdk deploy --require-approval never

# Rollback if needed
npx cdk deploy --rollback
```

### Kubernetes Cluster Upgrades

```bash
# Update cluster control plane
aws eks update-cluster-version \
  --name llm-proxy-cluster \
  --kubernetes-version 1.29

# Update node groups
aws eks update-nodegroup-version \
  --cluster-name llm-proxy-cluster \
  --nodegroup-name compute \
  --kubernetes-version 1.29
```

### Application Updates

```bash
# Update Helm chart
helm upgrade llm-proxy ../../helm/llm-proxy \
  --namespace llm-proxy \
  --values examples/values-production.yaml

# Rollback application
helm rollback llm-proxy 1 -n llm-proxy
```

## Cost Optimization

### Development Environment

- Use smaller instance types (t3.medium)
- Disable Redis persistence
- Single replica deployments
- Spot instances for non-critical workloads

### Production Environment

- Use appropriate instance types (m5.large+)
- Enable cluster autoscaler
- Use reserved instances for baseline capacity
- Monitor and optimize resource requests/limits

## Troubleshooting

### Common Issues

#### EKS Cluster Access

```bash
# Update kubeconfig
aws eks update-kubeconfig --region us-west-2 --name llm-proxy-cluster

# Check AWS credentials
aws sts get-caller-identity

# Verify cluster status
aws eks describe-cluster --name llm-proxy-cluster
```

#### Helm Deployment Issues

```bash
# Check Helm release
helm status llm-proxy -n llm-proxy

# View Helm values
helm get values llm-proxy -n llm-proxy

# Debug template rendering
helm template llm-proxy ../../helm/llm-proxy --debug
```

#### Networking Issues

```bash
# Check security groups
aws ec2 describe-security-groups \
  --filters "Name=group-name,Values=*llm-proxy*"

# Verify VPC endpoints
aws ec2 describe-vpc-endpoints

# Test DNS resolution
kubectl run debug --image=busybox --rm -it -- nslookup kubernetes.default
```

### Cleanup

```bash
# Delete CDK stack
npx cdk destroy

# Verify cleanup
aws cloudformation list-stacks \
  --query 'StackSummaries[?contains(StackName, `LlmProxy`)]'

# Manual cleanup (if needed)
aws eks delete-cluster --name llm-proxy-cluster
```

## Security Considerations

### Network Security

- Private node groups by default
- Network policies for pod-to-pod communication
- Security groups with minimal required access
- VPC endpoints for AWS services

### Access Control

- RBAC with least-privilege service accounts
- IRSA for AWS service access
- External secrets for sensitive data
- Pod security contexts and standards

### Monitoring

- CloudTrail for API auditing
- VPC Flow Logs for network monitoring
- CloudWatch for application logs
- Security scanning with tools like Falco

## Support

For issues and questions:
- **GitHub Issues**: https://github.com/sofatutor/llm-proxy/issues
- **AWS Documentation**: https://docs.aws.amazon.com/eks/
- **CDK Documentation**: https://docs.aws.amazon.com/cdk/