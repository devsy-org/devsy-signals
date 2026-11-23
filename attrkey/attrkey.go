// Package attrkey holds the shared slog attribute keys. Keys are lowercase,
// dot-separated snake_case. Where an OpenTelemetry semantic convention key
// exists, it is used as is; Devsy keys start with "devsy.".
//
// Never log url.full, header values, prompts, request or response bodies,
// file contents, or command arguments after argv[0].
package attrkey

// OpenTelemetry semantic convention keys.
const (
	ServiceVersion         = "service.version"
	OSType                 = "os.type"
	HostArch               = "host.arch"
	ProcessExitCode        = "process.exit.code"
	ErrorType              = "error.type"
	ServerAddress          = "server.address"
	URLScheme              = "url.scheme"
	HTTPRequestMethod      = "http.request.method"
	HTTPResponseStatusCode = "http.response.status_code"
	ContainerID            = "container.id"
	K8sNamespaceName       = "k8s.namespace.name"
)

// Devsy keys.
const (
	// ProviderName is the provider name as configured.
	ProviderName = "devsy.provider.name"
	// ProviderVersion is the provider release version.
	ProviderVersion = "devsy.provider.version"
	// Command is the cobra command path, without arguments.
	Command = "devsy.command"
	// Phase is the lifecycle phase of the current operation.
	Phase = "devsy.phase"
	// DurationMS is an elapsed time in whole milliseconds.
	DurationMS = "devsy.duration_ms"
	// Raw marks a record wrapped from a plain text line.
	Raw = "devsy.raw"
	// Truncated marks a record whose value or line was cut to its size limit.
	Truncated = "devsy.truncated"
)
