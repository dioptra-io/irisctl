# CLI

irisctl [--brief] [--curl] [--no-delete] [--no-auto-login] [--verbose] <command>

agents [--tag]
agents [<agent>...]

analyze [--before] [--after] [--state <state>] [--tag <tag>] [--tags-and] [--links-file <file>]
analyze hours [--chart]
analyze tags
analyze states

auth <subcommand>
auth login [--cookie]
auth logout [--cookie]
auth register <user-details>...

check <subcommand>
check agents [--uptime] [--net]
check containers [--errors] [--logs]

maint <subcommand>
maint dq <queue-name>...
maint dq --upload <queue-name> <actor-string>
maint dq --delete <queue-name> <redis-message-id>
maint delete <meas-uuid>

meas [--state <state>] [--tag <tag>] [--all] [--public]
meas --uuid <meas-uuid>...
meas --target-list <meas-uuid> <agent-uuid>
meas request <meas-md-file>...
meas delete <meas-uuid>...
meas edit <meas-uuid> <meas-md-file>

status (has no flags)

targets <subcommand>
targets all
targets [--with-conent] key <key>...
targets upload [--probe] <file>
targets delete <key>

users <subcommand>
users me
users all [--verified]
users delete [--dry-run] <user-id>...
users patch <user-id> <user-details>
users services <meas-uuid>

# Naming Conventions

## Function names
	Functions specified in the Run field of cobra.Command:
	command: <cmdName>
	subcommand: <cmdName><subcmdName>
	
## Flag names
	f<subcmd><flag> // irisctl monitor agents --status => fAgentStatus


## cobra.Command Initialization

## Usage() and Help() functions should be set where the command is defined.  For example:

        agentsCmd := &cobra.Command{
		...
        }
        agentsCmd.SetUsageFunc(common.Usage)
        agentsCmd.SetHelpFunc(common.Help)

## Commands or subcommands that accept arguments should initialize Command.Args as follows:

   Args: cmdNameSubcmdNameArgs,

   For example:

	func usersDeleteArgs(cmd *cobra.Command, args []string) error {
		if len(args) == 2 && args[0] == "usage" {
			fmt.Printf(args[1], "<user-id>...", "one or more user IDs")
			return nil
		}
		if len(args) < 1 {
			cliFatal("users delete requires at least one argument: <user-id>")
		}
		common.ValidateFormat(args, common.UserID)
		return nil
	}

## Fatal Functions
- cliFatal()
- fatal()
