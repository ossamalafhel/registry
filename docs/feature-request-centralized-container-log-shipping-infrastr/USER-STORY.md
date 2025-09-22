# Product Analysis

## Feature Title
Centralized Container Log Shipping Infrastructure

## User Story
As a DevOps engineer, I want to view all container logs in a central location so that I can quickly detect and debug production issues.

## Business Purpose
Reduce Mean Time To Detect (MTTD) from unknown to <5 minutes for critical production issues by centralizing container log collection, enabling faster incident response and improved system reliability.

## Stakeholders
1. **DevOps/SRE Team**: Need centralized log access with fast search capabilities to debug issues and monitor system health
2. **Engineering Team**: Need quick access to application logs for debugging and troubleshooting without SSH access to individual containers

## Success Metrics
1. **MTTD Reduction**: <5 minutes detection time - Critical for minimizing downtime and user impact
2. **Query Performance**: <3 seconds for 24-hour log searches - Ensures efficient debugging during incidents
3. **Coverage**: 100% container log collection - Complete visibility across production environment

## Risks and Dependencies
1. **Storage Costs**: High log volume could exceed budget - Address with aggressive filtering (exclude healthchecks) and 7-day retention policy
   - Affected repositories: https://github.com/ossamalafhel/registry
   
2. **Performance Impact**: DaemonSet resource consumption on nodes - Mitigate with strict resource limits (100m CPU, 128Mi memory) and monitoring

Dependencies:
- Existing Kubernetes cluster
- Deployed Grafana instance
- Pulumi infrastructure setup

## MVP Scope (5 days)
**Day 1-2**: Deploy OpenTelemetry Collector DaemonSet + Loki to staging
**Day 3**: Production deployment with conservative limits
**Day 4-5**: Add Grafana dashboards, basic filtering, and alerts

**Technical Approach**:
- OpenTelemetry Collector as DaemonSet for log collection
- Loki for storage (single binary mode, 30GB PVC)
- Grafana integration with pre-built dashboards
- TLS encryption and RBAC for security

**Out of Scope for MVP**:
- Complex log parsing/enrichment
- Multi-region setup
- Advanced alerting rules
- PII filtering (add if compliance required)

## Additional Context
Follow existing Pulumi patterns in `deploy/pkg/k8s/`. Consider VictoriaLogs as alternative if better integration with existing VictoriaMetrics is needed. Start with minimal configuration and iterate based on actual usage patterns.