export type AdvOption = {
  key: string;
  group: string;
  kind: "text" | "enum";
  options?: string[];
  defaultValue: string;
  /** Quick Help text shown when this option is selected (from ssh_config(5)). */
  help?: string;
};

export const ADVANCED_OPTIONS: AdvOption[] = [
  {
    key: "AddressFamily",
    group: "Basic",
    kind: "enum",
    options: ["any", "inet", "inet6"],
    defaultValue: "any",
    help: "Specifies which address family to use when connecting. Valid arguments are any (the default), inet (use IPv4 only), or inet6 (use IPv6 only).",
  },
  {
    key: "BindAddress",
    group: "Basic",
    kind: "text",
    defaultValue: "",
    help: "Use the specified address on the local machine as the source address of the connection. Only useful on systems with more than one address.",
  },
  {
    key: "BindInterface",
    group: "Basic",
    kind: "text",
    defaultValue: "",
    help: "Use the address of the specified interface on the local machine as the source address of the connection.",
  },
  {
    key: "Compression",
    group: "Basic",
    kind: "enum",
    options: ["yes", "no"],
    defaultValue: "no",
    help: "Specifies whether to use compression. The argument must be yes or no (the default). Compression applies to all traffic that flows over the SSH connection. If untrusted traffic (such as an open port-forward) is permitted over the connection alongside trusted traffic, then compression may leak information about session contents. For this reason, it is not recommended to enable compression for connections that share trusted and untrusted traffic.",
  },
  {
    key: "EscapeChar",
    group: "Basic",
    kind: "text",
    defaultValue: "~",
    help: "Sets the escape character (default: '~'). The escape character can also be set on the command line. The argument should be a single character, '^' followed by a letter, or none to disable the escape character entirely (making the connection transparent for binary data).",
  },
  {
    key: "EnableEscapeCommandline",
    group: "Basic",
    kind: "enum",
    options: ["yes", "no"],
    defaultValue: "no",
    help: "Enables the command line option in the EscapeChar menu for interactive sessions (default '~C'). By default, the command line is disabled.",
  },
  {
    key: "IPQoS",
    group: "Basic",
    kind: "text",
    defaultValue: "af21 cs1",
    help: "Specifies the Differentiated Services Field Codepoint (DSCP) value for connections. Accepted values are af11, af12, af13, af21, af22, af23, af31, af32, af33, af41, af42, af43, cs0, cs1, cs2, cs3, cs4, cs5, cs6, cs7, ef, le, a numeric value, or none to use the operating system default. This option may take one or two arguments, separated by whitespace. If one argument is specified, it is used as the packet class unconditionally. If two values are specified, the first is automatically selected for interactive sessions and the second for non-interactive sessions. The default is ef (Expedited Forwarding) for interactive sessions and none (the operating system default) for non-interactive sessions.",
  },
  {
    key: "LogLevel",
    group: "Basic",
    kind: "enum",
    options: [
      "QUIET",
      "FATAL",
      "ERROR",
      "INFO",
      "VERBOSE",
      "DEBUG",
      "DEBUG1",
      "DEBUG2",
      "DEBUG3",
    ],
    defaultValue: "INFO",
    help: "Gives the verbosity level that is used when logging messages from ssh(1). The possible values are: QUIET, FATAL, ERROR, INFO, VERBOSE, DEBUG, DEBUG1, DEBUG2, and DEBUG3. The default is INFO. DEBUG and DEBUG1 are equivalent. DEBUG2 and DEBUG3 each specify higher levels of verbose output.",
  },
  {
    key: "SendEnv",
    group: "Basic",
    kind: "text",
    defaultValue: "LC_*",
    help: "Specifies what variables from the local environ(7) should be sent to the server. The server must also support it, and the server must be configured to accept these environment variables. Note that the TERM environment variable is always sent whenever a pseudo-terminal is requested as it is required by the protocol. Refer to AcceptEnv in sshd_config(5) for how to configure the server. Variables are specified by name, which may contain wildcard characters. Multiple environment variables may be separated by whitespace or spread across multiple SendEnv directives. See PATTERNS for more information on patterns. It is possible to clear previously set SendEnv variable names by prefixing patterns with -. The default is not to send any environment variables.",
  },
  {
    key: "SetEnv",
    group: "Basic",
    kind: "text",
    defaultValue: "TERM=xterm-256color",
    help: "Directly specify one or more environment variables and their contents to be sent to the server in the form “NAME=VALUE”. Similarly to SendEnv, with the exception of the TERM variable, the server must be prepared to accept the environment variable. The VALUE may use tokens and environment variables as described in ssh_config(5).",
  },

  {
    key: "ChannelTimeout",
    group: "Activity",
    kind: "text",
    defaultValue: "none",
    help: "Specifies whether and how quickly ssh(1) should close inactive channels. Timeouts are specified as one or more “type=interval” pairs separated by whitespace, where the type must be the special keyword “global” or a channel type name (agent-connection, direct-tcpip, direct-streamlocal@openssh.com, forwarded-tcpip, forwarded-streamlocal@openssh.com, session, tun-connection, x11-connection), optionally containing wildcard characters. The interval is specified in seconds or may use TIME FORMATS units (e.g. session=5m). Specifying a zero value disables the inactivity timeout. The special timeout “global” applies to all active channels taken together. The default is not to expire channels of any type for inactivity.",
  },
  {
    key: "ConnectTimeout",
    group: "Activity",
    kind: "text",
    defaultValue: "none",
    help: "Specifies the timeout (in seconds) used when connecting to the SSH server, instead of using the default system TCP timeout. This timeout is applied both to establishing the connection and to performing the initial SSH protocol handshake and key exchange.",
  },
  {
    key: "ObscureKeystrokeTiming",
    group: "Activity",
    kind: "enum",
    options: ["yes", "no"],
    defaultValue: "yes",
    help: "Specifies whether ssh(1) should try to obscure inter-keystroke timings from passive observers of network traffic. If enabled, then for interactive sessions, ssh(1) will send keystrokes at fixed intervals of a few tens of milliseconds and will send fake keystroke packets for some time after typing ceases. The argument must be yes, no or an interval specifier of the form interval:milliseconds (e.g. interval:80). The default is to obscure keystrokes using a 20ms packet interval. Note that smaller intervals will result in higher fake keystroke packet rates.",
  },
  {
    key: "RekeyLimit",
    group: "Activity",
    kind: "text",
    defaultValue: "0 0",
    help: "Specifies the maximum amount of data that may be transmitted or received before the session key is renegotiated, optionally followed by a maximum amount of time that may pass before the session key is renegotiated. The first argument is specified in bytes and may have a suffix of ‘K’, ‘M’, or ‘G’. The default is between ‘1G’ and ‘4G’, depending on the cipher. The optional second value is specified in seconds or TIME FORMATS units. The default value is “default none”, which means that rekeying is performed after the cipher's default amount of data has been sent or received and no time based rekeying is done.",
  },
  {
    key: "ServerAliveCountMax",
    group: "Activity",
    kind: "text",
    defaultValue: "3",
    help: "Sets the number of server alive messages which may be sent without ssh(1) receiving any messages back from the server. If this threshold is reached while server alive messages are being sent, ssh will disconnect from the server, terminating the session. Server alive messages are sent through the encrypted channel and therefore will not be spoofable (unlike TCPKeepAlive). The default value is 3. If, for example, ServerAliveInterval is set to 15 and ServerAliveCountMax is left at the default, if the server becomes unresponsive, ssh will disconnect after approximately 45 seconds.",
  },
  {
    key: "ServerAliveInterval",
    group: "Activity",
    kind: "text",
    defaultValue: "0",
    help: "Sets a timeout interval in seconds after which if no data has been received from the server, ssh(1) will send a message through the encrypted channel to request a response from the server. The default is 0, indicating that these messages will not be sent to the server.",
  },
  {
    key: "TCPKeepAlive",
    group: "Activity",
    kind: "enum",
    options: ["yes", "no"],
    defaultValue: "yes",
    help: "Specifies whether the system should send keepalive messages on TCP sockets it has opened. If they are sent, failure of the connection or crash of one of the endpoints may be more promptly detected. However, this means that connections may terminate if the connection suffers a transient disruption. The argument must be transport to enable TCP keepalive messages on the SSH transport connection, yes (an alias for transport), all to enable TCP keepalive on all sockets opened by ssh(1) including X11 or port forwarding, or no to disable them. The default is transport. See also ServerAliveInterval for a more robust connection failure detection mechanism at the SSH protocol level.",
  },

  {
    key: "ExitOnForwardFailure",
    group: "Forwarding",
    kind: "enum",
    options: ["yes", "no"],
    defaultValue: "no",
    help: "Specifies whether ssh(1) should terminate the connection if it cannot set up all requested dynamic, tunnel, local, and remote port forwardings (e.g. if either end is unable to bind and listen on a specified port). Note that ExitOnForwardFailure does not apply to connections made over port forwardings and will not, for example, cause ssh(1) to exit if TCP connections to the ultimate forwarding destination fail. The argument must be yes or no (the default).",
  },
  {
    key: "ForwardX11",
    group: "Forwarding",
    kind: "enum",
    options: ["yes", "no"],
    defaultValue: "no",
    help: "Specifies whether X11 connections will be automatically redirected over the secure channel and DISPLAY set. The argument must be yes or no (the default). X11 forwarding should be enabled with caution. Users with the ability to bypass file permissions on the remote host (for the user's X11 authorization database) can access the local X11 display through the forwarded connection. An attacker may then be able to perform activities such as keystroke monitoring if the ForwardX11Trusted option is also enabled.",
  },
  {
    key: "ForwardX11Timeout",
    group: "Forwarding",
    kind: "text",
    defaultValue: "1200",
    help: "Specify a timeout for untrusted X11 forwarding using the format described in the TIME FORMATS section of sshd_config(5). X11 connections received by ssh(1) after this time will be refused. Setting ForwardX11Timeout to zero will disable the timeout and permit X11 forwarding for the life of the connection. The default is to disable untrusted X11 forwarding after twenty minutes has elapsed.",
  },
  {
    key: "GatewayPorts",
    group: "Forwarding",
    kind: "enum",
    options: ["yes", "no", "clientspecified"],
    defaultValue: "no",
    help: "Specifies whether remote hosts are allowed to connect to local forwarded ports. By default, ssh(1) binds local port forwardings to the loopback address. This prevents other remote hosts from connecting to forwarded ports. GatewayPorts can be used to specify that ssh should bind local port forwardings to the wildcard address, thus allowing remote hosts to connect to forwarded ports. The argument must be yes or no (the default).",
  },
  {
    key: "LocalForward",
    group: "Forwarding",
    kind: "text",
    defaultValue: "",
    help: "Specifies that a TCP port or Unix-domain socket on the local machine be forwarded over the secure channel to the specified host and port (or Unix-domain socket) from the remote machine. For a TCP port, the first argument must be [bind_address:]port or a Unix domain socket path. The second argument is the destination and may be host:hostport or a Unix domain socket path if the remote host supports it. IPv6 addresses can be specified by enclosing addresses in square brackets. If either argument contains a '/' in it, that argument will be interpreted as a Unix-domain socket rather than a TCP port. Multiple forwardings may be specified. Only the superuser can forward privileged ports. By default, the local port is bound in accordance with the GatewayPorts setting.",
  },
  {
    key: "RemoteForward",
    group: "Forwarding",
    kind: "text",
    defaultValue: "",
    help: "Specifies that a TCP port or Unix-domain socket on the remote machine be forwarded over the secure channel. The remote port may either be forwarded to a specified host and port or Unix-domain socket from the local machine, or may act as a SOCKS 4/5 proxy that allows a remote client to connect to arbitrary destinations from the local machine. The first argument is the listening specification ([bind_address:]port or a Unix domain socket path). If forwarding to a specific destination then the second argument must be host:hostport or a Unix domain socket path; otherwise if no destination is specified then the remote forwarding will be established as a SOCKS proxy (restrictable with PermitRemoteOpen). If the port argument is 0, the listen port will be dynamically allocated on the server. Specifying a remote bind_address will only succeed if the server's GatewayPorts option is enabled.",
  },

  {
    key: "StrictHostKeyChecking",
    group: "Security",
    kind: "enum",
    options: ["yes", "no", "ask", "accept-new"],
    defaultValue: "ask",
    help: "If this flag is set to yes, ssh(1) will never automatically add host keys to the ~/.ssh/known_hosts file, and refuses to connect to hosts whose host key has changed. This provides maximum protection against man-in-the-middle (MITM) attacks. If set to accept-new then ssh will automatically add new host keys to the user's known_hosts file, but will not permit connections to hosts with changed host keys. If set to no or off, ssh will automatically add new host keys and allow connections to hosts with changed hostkeys to proceed, subject to some restrictions. If set to ask (the default), new host keys will be added only after the user has confirmed that is what they really want to do, and ssh will refuse to connect to hosts whose host key has changed. The host keys of known hosts will be verified automatically in all cases.",
  },
  {
    key: "HashKnownHosts",
    group: "Security",
    kind: "enum",
    options: ["yes", "no"],
    defaultValue: "no",
    help: "Indicates that ssh(1) should hash host names and addresses when they are added to ~/.ssh/known_hosts. These hashed names may be used normally by ssh(1) and sshd(8), but they do not visually reveal identifying information if the file's contents are disclosed. The default is no. Note that existing names and addresses in known hosts files will not be converted automatically, but may be manually hashed using ssh-keygen(1).",
  },
  {
    key: "UserKnownHostsFile",
    group: "Security",
    kind: "text",
    defaultValue: "",
    help: "Specifies one or more files to use for the user host key database, separated by whitespace. Each filename may use tilde notation to refer to the user's home directory, tokens, and environment variables as described in ssh_config(5). A value of none causes ssh(1) to ignore any user-specific known hosts files. The default is ~/.ssh/known_hosts, ~/.ssh/known_hosts2.",
  },
  {
    key: "IdentitiesOnly",
    group: "Security",
    kind: "enum",
    options: ["yes", "no"],
    defaultValue: "no",
    help: "Specifies that ssh(1) should only use the configured authentication identity and certificate files (either the default files, or those explicitly configured in the ssh_config files or passed on the ssh(1) command-line), even if ssh-agent(1) or a PKCS11Provider or SecurityKeyProvider offers more identities. The argument must be yes or no (the default). This option is intended for situations where ssh-agent offers many different identities.",
  },
];

export function groupOptions(opts: AdvOption[]): Record<string, AdvOption[]> {
  const out: Record<string, AdvOption[]> = {};
  for (const o of opts) {
    (out[o.group] ??= []).push(o);
  }
  return out;
}

export function findOption(key: string | null): AdvOption | undefined {
  if (!key) return undefined;
  return ADVANCED_OPTIONS.find((o) => o.key === key);
}
