# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| CVSS v3.0 | Supported Versions                        |
| --------- | ----------------------------------------- |
| 9.0-10.0  | Releases within the previous three months |
| 4.0-8.9   | Most recent release                       |

## Reporting a Vulnerability

The Kogaro team and community take security bugs seriously. We appreciate your efforts to responsibly disclose your findings, and will make every effort to acknowledge your contributions.

To report a security issue, please use the GitHub Security Advisory ["Report a Vulnerability"](https://github.com/topiaruss/kogaro/security/advisories/new) tab.

The Kogaro team will send a response indicating the next steps in handling your report. After the initial reply to your report, the security team will keep you informed of the progress towards a fix and full announcement, and may ask for additional information or guidance.

### Security Bug Bounty

We do not currently offer a security bug bounty program.

## Security Considerations

### Cluster Permissions

Kogaro requires read-only access to various Kubernetes resources to perform validation. The principle of least privilege should be applied when deploying Kogaro:

- Grant only the minimum required RBAC permissions
- Use dedicated ServiceAccounts with limited scope
- Consider namespace isolation for sensitive workloads

### Data Handling

Kogaro does not:
- Store or transmit sensitive data from your cluster
- Log resource contents or secret values
- Retain historical data beyond metrics retention

### Deployment Security

When deploying Kogaro:
- Use container image verification
- Apply Pod Security Standards
- Enable network policies if applicable
- Regularly update to the latest version

## Known Security Considerations

### False Positives

Kogaro may report validation errors for resources in transitional states. This is by design and should not be considered a security issue.

### Resource Access

Kogaro requires broad read access to cluster resources. This access is necessary for comprehensive validation but should be audited in security-sensitive environments.

## Contact

For security-related questions or concerns that don't warrant a vulnerability report, please open an issue in the repository or contact the maintainers directly.