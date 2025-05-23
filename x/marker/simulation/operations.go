package simulation

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"time"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/cosmos/cosmos-sdk/x/bank/testutil"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/cosmos/cosmos-sdk/x/simulation"

	simappparams "github.com/provenance-io/provenance/app/params"
	"github.com/provenance-io/provenance/x/marker/keeper"
	"github.com/provenance-io/provenance/x/marker/types"
)

// Simulation operation weights constants
const (
	//nolint:gosec // not credentials
	OpWeightMsgAddMarker = "op_weight_msg_add_marker"
	//nolint:gosec // not credentials
	OpWeightMsgChangeStatus = "op_weight_msg_change_status"
	//nolint:gosec // not credentials
	OpWeightMsgAddAccess = "op_weight_msg_add_access"
	//nolint:gosec // not credentials
	OpWeightMsgAddActivateFinalizeMarker = "op_weight_msg_add_finalize_activate_marker"
	//nolint:gosec // not credentials
	OpWeightMsgAddMarkerProposal = "op_weight_msg_add_marker_proposal"
	//nolint:gosec // not credentials
	OpWeightMsgSetAccountData = "op_weight_msg_set_account_data"
	//nolint:gosec // not credentials
	OpWeightMsgUpdateSendDenyList = "op_weight_msg_update_send_deny_list"
)

// WeightedOperations returns all the operations from the module with their respective weights
func WeightedOperations(
	simState module.SimulationState, protoCodec *codec.ProtoCodec,
	k keeper.Keeper, ak authkeeper.AccountKeeper, bk bankkeeper.Keeper, gk govkeeper.Keeper, attrk types.AttrKeeper,
) simulation.WeightedOperations {
	args := &WeightedOpsArgs{
		SimState:   simState,
		ProtoCodec: protoCodec,
		AK:         ak,
		BK:         bk,
		GK:         gk,
		AttrK:      attrk,
	}

	var (
		wMsgAddMarker          int
		wMsgChangeStatus       int
		wMsgAddAccess          int
		wMsgAFAM               int
		wMsgAddMarkerProposal  int
		wMsgSetAccountData     int
		wMsgUpdateSendDenyList int
	)

	simState.AppParams.GetOrGenerate(OpWeightMsgAddMarker, &wMsgAddMarker, nil,
		func(_ *rand.Rand) { wMsgAddMarker = simappparams.DefaultWeightMsgAddMarker })
	simState.AppParams.GetOrGenerate(OpWeightMsgChangeStatus, &wMsgChangeStatus, nil,
		func(_ *rand.Rand) { wMsgChangeStatus = simappparams.DefaultWeightMsgChangeStatus })
	simState.AppParams.GetOrGenerate(OpWeightMsgAddAccess, &wMsgAddAccess, nil,
		func(_ *rand.Rand) { wMsgAddAccess = simappparams.DefaultWeightMsgAddAccess })
	simState.AppParams.GetOrGenerate(OpWeightMsgAddActivateFinalizeMarker, &wMsgAFAM, nil,
		func(_ *rand.Rand) { wMsgAFAM = simappparams.DefaultWeightMsgAddFinalizeActivateMarker })
	simState.AppParams.GetOrGenerate(OpWeightMsgAddMarkerProposal, &wMsgAddMarkerProposal, nil,
		func(_ *rand.Rand) { wMsgAddMarkerProposal = simappparams.DefaultWeightMsgAddMarkerProposal })
	simState.AppParams.GetOrGenerate(OpWeightMsgSetAccountData, &wMsgSetAccountData, nil,
		func(_ *rand.Rand) { wMsgSetAccountData = simappparams.DefaultWeightMsgSetAccountData })
	simState.AppParams.GetOrGenerate(OpWeightMsgUpdateSendDenyList, &wMsgUpdateSendDenyList, nil,
		func(_ *rand.Rand) { wMsgUpdateSendDenyList = simappparams.DefaultWeightMsgUpdateDenySendList })

	return simulation.WeightedOperations{
		simulation.NewWeightedOperation(wMsgAddMarker, SimulateMsgAddMarker(k, args)),
		simulation.NewWeightedOperation(wMsgChangeStatus, SimulateMsgChangeStatus(k, args)),
		simulation.NewWeightedOperation(wMsgAddAccess, SimulateMsgAddAccess(k, args)),
		simulation.NewWeightedOperation(wMsgAFAM, SimulateMsgAddFinalizeActivateMarker(k, args)),
		simulation.NewWeightedOperation(wMsgAddMarkerProposal, SimulateMsgAddMarkerProposal(k, args)),
		simulation.NewWeightedOperation(wMsgSetAccountData, SimulateMsgSetAccountData(k, args)),
		simulation.NewWeightedOperation(wMsgUpdateSendDenyList, SimulateMsgUpdateSendDenyList(k, args)),
	}
}

// SimulateMsgAddMarker will Add a random marker with random configuration.
func SimulateMsgAddMarker(k keeper.Keeper, args *WeightedOpsArgs) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		mgrAccount, _ := simtypes.RandomAcc(r, accs)
		denom := randomUnrestrictedDenom(r, k.GetUnrestrictedDenomRegex(ctx))
		msg := types.NewMsgAddMarkerRequest(
			denom,
			sdkmath.NewIntFromBigInt(sdkmath.ZeroInt().BigInt().Rand(r, k.GetMaxSupply(ctx).BigInt())),
			simAccount.Address,
			mgrAccount.Address,
			randMarkerType(r), // coin or restricted_coin
			r.Intn(2) > 0,     // fixed supply
			r.Intn(2) > 0,     // allow gov
			r.Intn(2) > 0,     // allow forced transfer
			[]string{},
			0,
			0,
		)

		return Dispatch(r, app, ctx, args.SimState, args.AK, args.BK, simAccount, chainID, msg, nil)
	}
}

// SimulateMsgChangeStatus will randomly change the status of the marker depending on it's current state.
func SimulateMsgChangeStatus(k keeper.Keeper, args *WeightedOpsArgs) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		m := randomMarker(r, ctx, k)
		if m == nil {
			return simtypes.NoOpMsg(types.ModuleName, "ChangeStatus", "unable to get marker for status change"), nil, nil
		}
		var simAccount simtypes.Account
		var found bool
		var msg sdk.Msg
		switch m.GetStatus() {
		// 50% chance of (re-)issuing a finalize or a 50/50 chance to cancel/activate.
		case types.StatusProposed, types.StatusFinalized:
			if r.Intn(10) < 5 {
				msg = types.NewMsgFinalizeRequest(m.GetDenom(), m.GetManager())
			} else {
				if r.Intn(10) < 5 {
					msg = types.NewMsgCancelRequest(m.GetDenom(), simAccount.Address)
				} else {
					msg = types.NewMsgActivateRequest(m.GetDenom(), m.GetManager())
				}
			}
			simAccount, found = simtypes.FindAccount(accs, m.GetManager())
			if !found {
				return simtypes.NoOpMsg(types.ModuleName, fmt.Sprintf("%T", msg), "manager account does not exist"), nil, nil
			}
		case types.StatusActive:
			simAccount, found = randomAccWithAccess(r, m, accs, types.Access_Delete)
			if !found {
				return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgCancelRequest{}), "no account has cancel access"), nil, nil
			}
			msg = types.NewMsgCancelRequest(m.GetDenom(), simAccount.Address)
		case types.StatusCancelled:
			simAccount, found = randomAccWithAccess(r, m, accs, types.Access_Delete)
			if !found {
				return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgDeleteRequest{}), "no account has delete access"), nil, nil
			}
			msg = types.NewMsgDeleteRequest(m.GetDenom(), simAccount.Address)
		case types.StatusDestroyed:
			return simtypes.NoOpMsg(types.ModuleName, "ChangeStatus", "marker status is destroyed"), nil, nil
		default:
			return simtypes.NoOpMsg(types.ModuleName, "", "unknown marker status"), nil, fmt.Errorf("unknown marker status: %#v", m)
		}

		return Dispatch(r, app, ctx, args.SimState, args.AK, args.BK, simAccount, chainID, msg, nil)
	}
}

// SimulateMsgAddAccess will Add a random access to an account.
func SimulateMsgAddAccess(k keeper.Keeper, args *WeightedOpsArgs) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		m := randomMarker(r, ctx, k)
		if m == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(&types.MsgAddAccessRequest{}), "unable to get marker for access change"), nil, nil
		}
		if !m.GetManager().Equals(sdk.AccAddress{}) {
			simAccount, _ = simtypes.FindAccount(accs, m.GetManager())
		}
		grants := randomAccessGrants(r, accs, 100, m.GetMarkerType())
		msg := types.NewMsgAddAccessRequest(m.GetDenom(), simAccount.Address, grants[0])
		return Dispatch(r, app, ctx, args.SimState, args.AK, args.BK, simAccount, chainID, msg, nil)
	}
}

// SimulateMsgAddFinalizeActivateMarker will bind a NAME under an existing name using a 40% probability of restricting it.
func SimulateMsgAddFinalizeActivateMarker(k keeper.Keeper, args *WeightedOpsArgs) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		mgrAccount, _ := simtypes.RandomAcc(r, accs)
		denom := randomUnrestrictedDenom(r, k.GetUnrestrictedDenomRegex(ctx))
		markerType := randMarkerType(r)
		// random access grants
		grants := randomAccessGrants(r, accs, 100, markerType)
		msg := types.NewMsgAddFinalizeActivateMarkerRequest(
			denom,
			sdkmath.NewIntFromBigInt(sdkmath.ZeroInt().BigInt().Rand(r, k.GetMaxSupply(ctx).BigInt())),
			simAccount.Address,
			mgrAccount.Address,
			markerType,
			r.Intn(2) > 0, // fixed supply
			r.Intn(2) > 0, // allow gov
			r.Intn(2) > 0, // allow forced transfer
			[]string{},
			grants,
			0,
			0,
		)

		if msg.MarkerType != types.MarkerType_RestrictedCoin {
			msg.AllowForcedTransfer = false
		}

		return Dispatch(r, app, ctx, args.SimState, args.AK, args.BK, simAccount, chainID, msg, nil)
	}
}

// SimulateMsgAddMarkerProposal will broadcast a Add random Marker Proposal.
func SimulateMsgAddMarkerProposal(k keeper.Keeper, args *WeightedOpsArgs) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		simAccount, _ := simtypes.RandomAcc(r, accs)
		denom := randomUnrestrictedDenom(r, k.GetUnrestrictedDenomRegex(ctx))

		markerStatus := types.MarkerStatus(r.Intn(3) + 1) //nolint:gosec // G115: 1-3 always fits in a uint32 (implicit cast).
		markerType := randMarkerType(r)
		msg := &types.MsgAddMarkerRequest{
			Amount: sdk.Coin{
				Denom:  denom,
				Amount: sdkmath.NewIntFromBigInt(sdkmath.ZeroInt().BigInt().Rand(r, k.GetMaxSupply(ctx).BigInt())),
			},
			Manager:                simAccount.Address.String(),
			FromAddress:            k.GetAuthority(),
			Status:                 markerStatus,
			MarkerType:             markerType,
			AccessList:             []types.AccessGrant{{Address: simAccount.Address.String(), Permissions: randomAccessTypes(r, markerType)}},
			SupplyFixed:            r.Intn(2) > 0,
			AllowGovernanceControl: true,
			AllowForcedTransfer:    r.Intn(2) > 0,
			RequiredAttributes:     nil,
		}
		if msg.Status == types.StatusActive {
			msg.Manager = ""
		}
		if msg.MarkerType != types.MarkerType_RestrictedCoin {
			msg.AllowForcedTransfer = false
		}

		// Get the governance min deposit needed
		govParams, err := args.GK.Params.Get(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "failed to get gov params"), nil, err
		}
		govMinDep := sdk.NewCoins(govParams.MinDeposit...)

		sender, _ := simtypes.RandomAcc(r, accs)

		msgArgs := &SendGovMsgArgs{
			WeightedOpsArgs: *args,
			R:               r,
			App:             app,
			Ctx:             ctx,
			Accs:            accs,
			ChainID:         chainID,
			Sender:          sender,
			Msg:             msg,
			Deposit:         govMinDep,
			Comment:         "marker",
			Title:           fmt.Sprintf("Add Marker %s", denom),
			Summary:         fmt.Sprintf("Create the %q marker.", denom),
		}

		skip, opMsg, err := SendGovMsg(msgArgs)

		if skip || err != nil {
			return opMsg, nil, err
		}

		proposalID, err := args.GK.ProposalID.Peek(ctx)
		if err != nil {
			return opMsg, nil, err
		}
		proposalID--

		votingPeriod := govParams.VotingPeriod
		fops := make([]simtypes.FutureOperation, len(accs))
		for i, acct := range accs {
			whenVote := ctx.BlockHeader().Time.Add(time.Duration(r.Int63n(int64(votingPeriod.Seconds()))) * time.Second)
			fops[i] = simtypes.FutureOperation{
				BlockTime: whenVote,
				Op:        OperationMsgVote(args, acct, proposalID, govtypes.OptionYes, msgArgs.Comment),
			}
		}

		return opMsg, fops, nil
	}
}

// SimulateMsgSetAccountData will set randomized account data to a marker.
func SimulateMsgSetAccountData(k keeper.Keeper, args *WeightedOpsArgs) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		msg := &types.MsgSetAccountDataRequest{}

		marker, signer := randomMarkerWithAccessSigner(r, ctx, k, accs, types.Access_Deposit)
		if marker == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "unable to find marker with a deposit signer"), nil, nil
		}

		msg.Denom = marker.GetDenom()
		msg.Signer = signer.Address.String()

		// 1 in 10 chance that the value stays "".
		// 9 in 10 chance that it will be between 1 and MaxValueLen characters.
		if r.Intn(10) != 0 {
			maxLen := min(args.AttrK.GetMaxValueLength(ctx), 500)
			strLen := r.Intn(int(maxLen)) + 1
			msg.Value = simtypes.RandStringOfLength(r, strLen)
		}

		return Dispatch(r, app, ctx, args.SimState, args.AK, args.BK, signer, chainID, msg, nil)
	}
}

// SimulateMsgUpdateSendDenyList will update random marker with denied send addresses.
func SimulateMsgUpdateSendDenyList(k keeper.Keeper, args *WeightedOpsArgs) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		msg := &types.MsgUpdateSendDenyListRequest{}

		marker, signer := randomMarkerWithAccessSigner(r, ctx, k, accs, types.Access_Transfer)
		if marker == nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "unable to find marker with a transfer signer"), nil, nil
		}

		rDenyAccounts := simtypes.RandomAccounts(r, 10)
		addDenyAddresses := make([]string, len(rDenyAccounts))
		for i := range rDenyAccounts {
			addDenyAddresses[i] = rDenyAccounts[i].Address.String()
		}

		msg.Denom = marker.GetDenom()
		msg.AddDeniedAddresses = addDenyAddresses
		msg.Authority = signer.Address.String()

		return Dispatch(r, app, ctx, args.SimState, args.AK, args.BK, signer, chainID, msg, nil)
	}
}

// Dispatch sends an operation to the chain using a given account/funds on account for fees.  Failures on the server side
// are handled as no-op msg operations with the error string as the status/response.
func Dispatch(
	r *rand.Rand,
	app *baseapp.BaseApp,
	ctx sdk.Context,
	simState module.SimulationState,
	ak authkeeper.AccountKeeperI,
	bk bankkeeper.Keeper,
	from simtypes.Account,
	chainID string,
	msg sdk.Msg,
	futures []simtypes.FutureOperation,
) (
	simtypes.OperationMsg,
	[]simtypes.FutureOperation,
	error,
) {
	account := ak.GetAccount(ctx, from.Address)
	spendable := bk.SpendableCoins(ctx, account.GetAddress())

	fees, err := simtypes.RandomFees(r, ctx, spendable)
	if err != nil {
		return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "unable to generate fees"), nil, err
	}
	// fund account with nhash for additional fees, if the account exists (100m stake)
	if sdk.MsgTypeURL(msg) == "/provenance.marker.v1.MsgAddMarkerRequest" && ak.GetAccount(ctx, account.GetAddress()) != nil {
		err = testutil.FundAccount(ctx, bk, account.GetAddress(), sdk.NewCoins(sdk.Coin{
			Denom:  "stake",
			Amount: sdkmath.NewInt(100_000_000_000_000),
		}))
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "unable to fund account with additional fee"), nil, err
		}
		fees = fees.Add(sdk.Coin{
			Denom:  "stake",
			Amount: sdkmath.NewInt(100_000_000_000_000),
		})
	}

	tx, err := simtestutil.GenSignedMockTx(
		r,
		simState.TxConfig,
		[]sdk.Msg{msg},
		fees,
		simtestutil.DefaultGenTxGas,
		chainID,
		[]uint64{account.GetAccountNumber()},
		[]uint64{account.GetSequence()},
		from.PrivKey,
	)
	if err != nil {
		return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), "unable to generate mock tx"), nil, err
	}

	_, _, err = app.SimDeliver(simState.TxConfig.TxEncoder(), tx)
	if err != nil {
		return simtypes.NoOpMsg(types.ModuleName, sdk.MsgTypeURL(msg), err.Error()), nil, nil
	}

	return simtypes.NewOperationMsg(msg, true, ""), futures, nil
}

// randomUnrestrictedDenom returns a randomized unrestricted denom string value.
func randomUnrestrictedDenom(r *rand.Rand, unrestrictedDenomExp string) string {
	exp := regexp.MustCompile(`\{(\d+),(\d+)\}`)
	matches := exp.FindStringSubmatch(unrestrictedDenomExp)
	if len(matches) != 3 {
		panic("expected two number as range expression in unrestricted denom expression")
	}
	minLen, _ := strconv.ParseInt(matches[1], 10, 32)
	maxLen, _ := strconv.ParseInt(matches[2], 10, 32)

	return simtypes.RandStringOfLength(r, int(randomInt63(r, maxLen-minLen)+minLen))
}

// randomAccessGrants generates random access grants for randomly selected accounts.
// Each account has a 30% chance of being chosen with a max of limit.
func randomAccessGrants(r *rand.Rand, accs []simtypes.Account, limit int, markerType types.MarkerType) (grants []types.AccessGrant) {
	// select random number of accounts ...
	for i := 0; i < len(accs); i++ {
		if r.Intn(10) < 3 {
			continue
		}
		// for each of the accounts selected, add a random set of permissions.
		grants = append(grants, *types.NewAccessGrant(accs[i].Address, randomAccessTypes(r, markerType)))
		if len(grants) >= limit {
			return
		}
	}
	return
}

// randomAccessTypes builds a list of access rights with a 40% chance of including each one
func randomAccessTypes(r *rand.Rand, markerType types.MarkerType) (result []types.Access) {
	access := []string{"mint", "burn", "deposit", "withdraw", "delete", "admin"}
	if markerType == types.MarkerType_RestrictedCoin {
		access = append(access, "transfer")
	}
	for i := 0; i < len(access); i++ {
		if r.Intn(10) < 4 {
			result = append(result, types.AccessByName(access[i]))
		}
	}
	return
}

// randomMarker returns a randomly selected marker from store
func randomMarker(r *rand.Rand, ctx sdk.Context, k keeper.Keeper) types.MarkerAccountI {
	var markers []types.MarkerAccountI
	k.IterateMarkers(ctx, func(marker types.MarkerAccountI) (stop bool) {
		markers = append(markers, marker)
		return false
	})
	if len(markers) == 0 {
		return nil
	}
	idx := r.Intn(len(markers))
	return markers[idx]
}

// randomMarkerWithAccessSigner returns a randomly selected marker and account that has specified access.
func randomMarkerWithAccessSigner(r *rand.Rand, ctx sdk.Context, k keeper.Keeper, accs []simtypes.Account, access types.Access) (types.MarkerAccountI, simtypes.Account) {
	var markers []types.MarkerAccountI
	k.IterateMarkers(ctx, func(marker types.MarkerAccountI) (stop bool) {
		markers = append(markers, marker)
		return false
	})
	if len(markers) == 0 {
		return nil, simtypes.Account{}
	}

	r.Shuffle(len(markers), func(i, j int) {
		markers[i], markers[j] = markers[j], markers[i]
	})

	for _, marker := range markers {
		acc, found := randomAccWithAccess(r, marker, accs, access)
		if found {
			return marker, acc
		}
	}

	return nil, simtypes.Account{}
}

func randomAccWithAccess(r *rand.Rand, marker types.MarkerAccountI, accs []simtypes.Account, access types.Access) (simtypes.Account, bool) {
	addrs := marker.AddressListForPermission(access)

	if len(addrs) == 0 {
		return simtypes.Account{}, false
	}

	r.Shuffle(len(addrs), func(i, j int) {
		addrs[i], addrs[j] = addrs[j], addrs[i]
	})

	for _, addr := range addrs {
		acc, found := simtypes.FindAccount(accs, addr)
		if found {
			return acc, true
		}
	}

	return simtypes.Account{}, false
}

func randomInt63(r *rand.Rand, maxVal int64) (result int64) {
	if maxVal == 0 {
		return 0
	}
	return r.Int63n(maxVal)
}

func randMarkerType(r *rand.Rand) types.MarkerType {
	return types.MarkerType(r.Intn(2) + 1) //nolint:gosec // G115: Either 1 or 2, so always fits in int32 (implicit cast).
}

// WeightedOpsArgs holds all the args provided to WeightedOperations so that they can be passed on later more easily.
type WeightedOpsArgs struct {
	SimState   module.SimulationState
	ProtoCodec *codec.ProtoCodec
	AK         authkeeper.AccountKeeperI
	BK         bankkeeper.Keeper
	GK         govkeeper.Keeper
	AttrK      types.AttrKeeper
}

// SendGovMsgArgs holds all the args available and needed for sending a gov msg.
type SendGovMsgArgs struct {
	WeightedOpsArgs

	R       *rand.Rand
	App     *baseapp.BaseApp
	Ctx     sdk.Context
	Accs    []simtypes.Account
	ChainID string

	Sender  simtypes.Account
	Msg     sdk.Msg
	Deposit sdk.Coins
	Comment string

	Title   string
	Summary string
}

// SendGovMsg sends a msg as a gov prop.
// It returns whether to skip the rest, an operation message, and any error encountered.
func SendGovMsg(args *SendGovMsgArgs) (bool, simtypes.OperationMsg, error) {
	msgType := sdk.MsgTypeURL(args.Msg)

	spendableCoins := args.BK.SpendableCoins(args.Ctx, args.Sender.Address)
	if spendableCoins.Empty() {
		return true, simtypes.NoOpMsg(types.ModuleName, msgType, "sender has no spendable coins"), nil
	}

	_, hasNeg := spendableCoins.SafeSub(args.Deposit...)
	if hasNeg {
		return true, simtypes.NoOpMsg(types.ModuleName, msgType, "sender has insufficient balance to cover deposit"), nil
	}

	msgAny, err := codectypes.NewAnyWithValue(args.Msg)
	if err != nil {
		return true, simtypes.NoOpMsg(types.ModuleName, msgType, "wrapping MsgAddMarkerProposalRequest as Any"), err
	}

	govMsg := &govtypes.MsgSubmitProposal{
		Messages:       []*codectypes.Any{msgAny},
		InitialDeposit: args.Deposit,
		Proposer:       args.Sender.Address.String(),
		Metadata:       "",
		Title:          args.Title,
		Summary:        args.Summary,
	}

	txCtx := simulation.OperationInput{
		R:               args.R,
		App:             args.App,
		TxGen:           args.SimState.TxConfig,
		Cdc:             args.ProtoCodec,
		Msg:             govMsg,
		CoinsSpentInMsg: govMsg.InitialDeposit,
		Context:         args.Ctx,
		SimAccount:      args.Sender,
		AccountKeeper:   args.AK,
		Bankkeeper:      args.BK,
		ModuleName:      types.ModuleName,
	}

	opMsg, _, err := simulation.GenAndDeliverTxWithRandFees(txCtx)
	if opMsg.Comment == "" {
		opMsg.Comment = args.Comment
	}

	return err != nil, opMsg, err
}

// OperationMsgVote returns an operation that casts a yes vote on a gov prop from an account.
func OperationMsgVote(args *WeightedOpsArgs, voter simtypes.Account, govPropID uint64, vote govtypes.VoteOption, comment string) simtypes.Operation {
	return func(
		r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context,
		_ []simtypes.Account, _ string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		msg := govtypes.NewMsgVote(voter.Address, govPropID, vote, "")

		txCtx := simulation.OperationInput{
			R:               r,
			App:             app,
			TxGen:           args.SimState.TxConfig,
			Cdc:             args.ProtoCodec,
			Msg:             msg,
			CoinsSpentInMsg: sdk.Coins{},
			Context:         ctx,
			SimAccount:      voter,
			AccountKeeper:   args.AK,
			Bankkeeper:      args.BK,
			ModuleName:      types.ModuleName,
		}

		opMsg, fops, err := simulation.GenAndDeliverTxWithRandFees(txCtx)
		if opMsg.Comment == "" {
			opMsg.Comment = comment
		}

		return opMsg, fops, err
	}
}
