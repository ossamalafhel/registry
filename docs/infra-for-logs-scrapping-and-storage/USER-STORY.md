# Product Analysis

## Feature Title
Container Log Shipping Infrastructure

## User Story
As a DevOps engineer, I want to ship and query container logs centrally so that I can quickly debug production issues and reduce MTTD.

## Business Purpose
Enable faster incident resolution by providing centralized log access, reducing Mean Time To Detect (MTTD) from hours to minutes for critical issues.

## Stakeholders
1. **DevOps/SRE Team**: Need real-time log access with proper resource tagging for debugging
2. **Engineering Team**: Need searchable logs to troubleshoot application issues

## Success Metrics
1. **MTTD Reduction**: <5 minutes for critical issues - Faster detection saves money and reputation
2. **Log Query Performance**: <3 seconds for 24-hour searches - Enables efficient debugging
3. **Log Coverage**: 100% of production containers shipping logs - Complete observability

## Risks and Dependencies
1. **Storage Costs**: High volume logs could increase infrastructure costs - Implement retention policies (7-30 days)
   - Affected repositories: https://github.com/ossamalafhel/registry
2. **Performance Impact**: Log collection might affect container performance - Use async shipping, monitor resource usage

## Additional Context
**Implementation Steps (MVP):**
1. Deploy OpenTelemetry collector as DaemonSet
2. Add resource processors for debugging context
3. Choose between Loki vs VictoriaLogs for storage
4. Integrate with existing Grafana for visualization

**Time Estimate**: 3-5 days
- Day 1-2: OTel collector setup + processors
- Day 3: Storage solution deployment
- Day 4-5: Grafana integration + testing

**Tech Stack**: Kubernetes, OpenTelemetry, Loki/VictoriaLogs, Grafana