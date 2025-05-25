# AWS EventBridge Connector (Optional)

## Summary
Implement an AWS EventBridge connector service. This component consumes events from the Redis-backed event bus and pushes them to AWS EventBridge, enabling routing to CloudWatch and other AWS-native services. It provides a cloud-native alternative to a dedicated dispatcher for AWS integrations.

## Rationale
- Leverages AWS EventBridge for scalable, managed event routing
- Enables integration with CloudWatch, Lambda, S3, and other AWS services without custom dispatcher code
- Reduces operational burden for AWS users
- Optional: can be used alongside or instead of dedicated dispatchers

## Requirements
- Service subscribes to the Redis-backed event bus
- Publishes events to a specified AWS EventBridge bus (using AWS SDK or HTTP API)
- Configurable AWS credentials, region, and event bus name
- Robust error handling and retry logic
- Metrics and logging for delivery status
- Documentation for setup and AWS permissions

## Tasks
- [ ] Design connector interface and configuration
- [ ] Implement Redis event bus consumer
- [ ] Implement AWS EventBridge publisher
- [ ] Add error handling, retries, and metrics
- [ ] Write tests for event delivery and failure scenarios
- [ ] Document setup, configuration, and AWS IAM requirements

## Acceptance Criteria
- Connector reliably consumes events from Redis and publishes to AWS EventBridge
- Delivery is robust, with retries and error handling
- Metrics and logs are available for monitoring
- Documentation covers setup and AWS integration
- Optional: can be enabled/disabled independently of other dispatchers 