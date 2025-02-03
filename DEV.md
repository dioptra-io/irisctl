The source code for `irisctl` follows specific conventions.
If you'd like to contribute, please adhere to these guidelines. 

# Naming Conventions

## Function names
	Functions specified in the Run field of cobra.Command:
	command: <cmdName>
	subcommand: <cmdName><subcmdName>
	
## Flag names
	f<subcmd><flag> // irisctl monitor agents --status => fAgentStatus


# cobra.Command Initialization

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

# Fatal Functions
	cliFatal() // invalid command line
	fatal() // errors that should terminate execution

# Annotations
	// TODO: mark incomplete work or improvements to be made
	// XXX: mark hacky code that needs a better solution but is left as-is for now
