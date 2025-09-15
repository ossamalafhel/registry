package model

// Registry Types - supported package registry types
const (
	RegistryTypeNPM   = "npm"
	RegistryTypePyPI  = "pypi"
	RegistryTypeOCI   = "oci"
	RegistryTypeNuGet = "nuget"
	RegistryTypeMCPB  = "mcpb"
)

// Registry Base URLs - supported package registry base URLs
const (
	RegistryURLNPM    = "https://registry.npmjs.org"
	RegistryURLPyPI   = "https://pypi.org"
	RegistryURLDocker = "https://docker.io"
	RegistryURLNuGet  = "https://api.nuget.org"
	RegistryURLGitHub = "https://github.com"
	RegistryURLGitLab = "https://gitlab.com"
	
	// Additional OCI registries
	RegistryURLGHCR          = "https://ghcr.io"
	RegistryURLGAR           = "https://artifactregistry.googleapis.com"
	RegistryURLGCR           = "https://gcr.io"
	RegistryURLECR           = "https://public.ecr.aws"
	RegistryURLACR           = "https://azurecr.io"
	RegistryURLQuay          = "https://quay.io"
	RegistryURLGitLabCR      = "https://registry.gitlab.com"
	RegistryURLDockerHub     = "https://hub.docker.com"
	RegistryURLJFrogCR       = "https://jfrog.io"
	RegistryURLHarborCR      = "https://goharbor.io"
	RegistryURLAlibabaACR    = "https://cr.console.aliyun.com"
	RegistryURLIBMCR         = "https://icr.io"
	RegistryURLOracleCR      = "https://container-registry.oracle.com"
	RegistryURLDigitalOceanCR = "https://registry.digitalocean.com"
)

// Transport Types - supported remote transport protocols
const (
	TransportTypeStreamableHTTP = "streamable-http"
	TransportTypeSSE            = "sse"
	TransportTypeStdio          = "stdio"
)

// Runtime Hints - supported package runtime hints
const (
	RuntimeHintNPX    = "npx"
	RuntimeHintUVX    = "uvx"
	RuntimeHintDocker = "docker"
	RuntimeHintDNX    = "dnx"
)