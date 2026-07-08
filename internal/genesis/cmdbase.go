package genesis

import (
	"context"
	"errors"
	"time"

	"github.com/cnrancher/hangar/pkg/signal"
	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/spf13/cobra"
)

var (
	signalContext context.Context = signal.SetupSignalContext()
	defaultUserAgent            = utils.DefaultUserAgent()
	ErrAborted                  = errors.New("aborted by user")
)

type baseCmd struct {
	*baseOpts
	cmd *cobra.Command
}

func newBaseCmd(cmd *cobra.Command) *baseCmd {
	return &baseCmd{cmd: cmd, baseOpts: &globalOpts}
}

type baseOpts struct {
	debug          bool
	policyPath     string
	insecurePolicy bool
	hideLogTime    bool
}

var globalOpts = baseOpts{}

func (cc *baseCmd) getCommand() *cobra.Command {
	return cc.cmd
}

func (cc *baseCmd) newSystemContext() *types.SystemContext {
	return &types.SystemContext{
		DockerRegistryUserAgent: defaultUserAgent,
	}
}

func (cc *baseCmd) getPolicy() (*signature.Policy, error) {
	var (
		policy *signature.Policy
		err    error
	)
	if cc.insecurePolicy {
		policy = &signature.Policy{
			Default: []signature.PolicyRequirement{
				signature.NewPRInsecureAcceptAnything(),
			},
			Transports: make(map[string]signature.PolicyTransportScopes),
		}
	} else if cc.policyPath == "" {
		policy, err = signature.DefaultPolicy(nil)
	} else {
		policy, err = signature.NewPolicyFromFile(cc.policyPath)
	}
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func (cc *baseCmd) ctxWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx := signalContext
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	return ctx, cancel
}
