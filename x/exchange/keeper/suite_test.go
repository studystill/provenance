package keeper_test

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	metadatatypes "github.com/provenance-io/provenance/x/metadata/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/gogoproto/proto"

	"github.com/provenance-io/provenance/app"
	"github.com/provenance-io/provenance/testutil/assertions"
	"github.com/provenance-io/provenance/x/exchange"
	"github.com/provenance-io/provenance/x/exchange/keeper"
	markertypes "github.com/provenance-io/provenance/x/marker/types"
)

type TestSuite struct {
	suite.Suite

	app *app.App
	ctx sdk.Context

	k keeper.Keeper

	addr1 sdk.AccAddress
	addr2 sdk.AccAddress
	addr3 sdk.AccAddress
	addr4 sdk.AccAddress
	addr5 sdk.AccAddress

	marketAddr1 sdk.AccAddress
	marketAddr2 sdk.AccAddress
	marketAddr3 sdk.AccAddress

	adminAddr sdk.AccAddress

	longAddr1 sdk.AccAddress
	longAddr2 sdk.AccAddress
	longAddr3 sdk.AccAddress

	feeCollector     string
	feeCollectorAddr sdk.AccAddress

	accKeeper *MockAccountKeeper

	logBuffer bytes.Buffer

	addrLookupMap map[string]string
}

func (s *TestSuite) SetupTest() {
	if s.addrLookupMap == nil {
		s.addrLookupMap = make(map[string]string)
	}

	// swap in the buffered logger maker so it's used in app.Setup, but then put it back (since that's a global thing).
	defer app.SetLoggerMaker(app.SetLoggerMaker(app.BufferedInfoLoggerMaker(&s.logBuffer)))

	s.app = app.Setup(s.T())
	s.logBuffer.Reset()
	s.ctx = s.app.BaseApp.NewContext(false)
	s.k = s.app.ExchangeKeeper

	addrs := app.AddTestAddrsIncremental(s.app, s.ctx, 5, sdkmath.NewInt(1_000_000_000))
	s.addr1 = addrs[0]
	s.addr2 = addrs[1]
	s.addr3 = addrs[2]
	s.addr4 = addrs[3]
	s.addr5 = addrs[4]
	s.addAddrLookup(s.addr1, "addr1")
	s.addAddrLookup(s.addr2, "addr2")
	s.addAddrLookup(s.addr3, "addr3")
	s.addAddrLookup(s.addr4, "addr4")
	s.addAddrLookup(s.addr5, "addr5")

	s.marketAddr1 = exchange.GetMarketAddress(1)
	s.marketAddr2 = exchange.GetMarketAddress(2)
	s.marketAddr3 = exchange.GetMarketAddress(3)
	s.addAddrLookup(s.marketAddr1, "marketAddr1")
	s.addAddrLookup(s.marketAddr2, "marketAddr2")
	s.addAddrLookup(s.marketAddr3, "marketAddr3")

	s.adminAddr = sdk.AccAddress("adminAddr___________")
	s.addAddrLookup(s.adminAddr, "adminAddr")

	longAddrs := app.AddTestAddrsIncrementalLong(s.app, s.ctx, 3, sdkmath.NewInt(1_000_000_000))
	s.longAddr1 = longAddrs[0]
	s.longAddr2 = longAddrs[1]
	s.longAddr3 = longAddrs[2]
	s.addAddrLookup(s.longAddr1, "longAddr1")
	s.addAddrLookup(s.longAddr2, "longAddr2")
	s.addAddrLookup(s.longAddr3, "longAddr3")

	s.feeCollector = s.k.GetFeeCollectorName()
	s.feeCollectorAddr = authtypes.NewModuleAddress(s.feeCollector)
	s.addAddrLookup(s.feeCollectorAddr, "feeCollectorAddr")
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

// sliceStrings converts each val into a string using the provided stringer, prefixing the slice index to each.
func sliceStrings[T any](vals []T, stringer func(T) string) []string {
	if vals == nil {
		return nil
	}
	strs := make([]string, len(vals))
	for i, v := range vals {
		strs[i] = fmt.Sprintf("[%d]:%s", i, stringer(v))
	}
	return strs
}

// sliceString converts each val into a string using the provided stringer with the index prefixed to it, and joins them with ", ".
func sliceString[T any](vals []T, stringer func(T) string) string {
	if vals == nil {
		return "<nil>"
	}
	return strings.Join(sliceStrings(vals, stringer), ", ")
}

// copySlice returns a copy of a slice using the provided copier for each value.
func copySlice[T any](vals []T, copier func(T) T) []T {
	if vals == nil {
		return nil
	}
	rv := make([]T, len(vals))
	for i, v := range vals {
		rv[i] = copier(v)
	}
	return rv
}

// noOpCopier is a passthrough "copier" function that just returns the exact same thing that was provided.
func noOpCopier[T any](val T) T {
	return val
}

// reverseSlice returns a new slice with the entries reversed.
func reverseSlice[T any](vals []T) []T {
	if vals == nil {
		return nil
	}
	rv := make([]T, len(vals))
	for i, val := range vals {
		rv[len(vals)-i-1] = val
	}
	return rv
}

// getLogOutput gets the log buffer contents. This (probably) also clears the log buffer.
func (s *TestSuite) getLogOutput(msg string, args ...interface{}) string {
	logOutput := s.logBuffer.String()
	s.T().Logf(msg+" log output:\n%s", append(args, logOutput)...)
	return logOutput
}

// splitOutputLog splits the given output log into its lines.
func (s *TestSuite) splitOutputLog(outputLog string) []string {
	if len(outputLog) == 0 {
		return nil
	}
	rv := strings.Split(outputLog, "\n")
	for len(rv) > 0 && len(rv[len(rv)-1]) == 0 {
		rv = rv[:len(rv)-1]
	}
	if len(rv) == 0 {
		return nil
	}
	return rv
}

// badKey creates a copy of the provided key, moves the last byte to the 2nd to last,
// then chops off the last byte (so the result is one byte shorter).
func (s *TestSuite) badKey(key []byte) []byte {
	rv := make([]byte, len(key)-1)
	copy(rv, key)
	rv[len(rv)-1] = key[len(key)-1]
	return rv
}

// coins creates a new sdk.Coins from a string, requiring it to work.
func (s *TestSuite) coins(coins string) sdk.Coins {
	s.T().Helper()
	rv, err := sdk.ParseCoinsNormalized(coins)
	s.Require().NoError(err, "ParseCoinsNormalized(%q)", coins)
	return rv
}

// coin creates a new coin from a string, requiring it to work.
func (s *TestSuite) coin(coin string) sdk.Coin {
	rv, err := sdk.ParseCoinNormalized(coin)
	s.Require().NoError(err, "ParseCoinNormalized(%q)", coin)
	return rv
}

// coinP creates a reference to a new coin from a string, requiring it to work.
func (s *TestSuite) coinP(coin string) *sdk.Coin {
	rv := s.coin(coin)
	return &rv
}

// coinsString converts a slice of coin entries into a string.
// This is different from sdk.Coins.String because the entries aren't sorted in here.
func (s *TestSuite) coinsString(coins []sdk.Coin) string {
	return sliceString(coins, func(coin sdk.Coin) string {
		return fmt.Sprintf("%q", coin)
	})
}

// coinPString converts the provided coin to a quoted string, or "<nil>".
func (s *TestSuite) coinPString(coin *sdk.Coin) string {
	if coin == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q", coin)
}

// ratio creates a FeeRatio from a "<price>:<fee>" string.
func (s *TestSuite) ratio(ratioStr string) exchange.FeeRatio {
	rv, err := exchange.ParseFeeRatio(ratioStr)
	s.Require().NoError(err, "ParseFeeRatio(%q)", ratioStr)
	return *rv
}

// ratios creates a slice of Fee ratio from a comma delimited list of "<price>:<fee>" entries in a string.
func (s *TestSuite) ratios(ratiosStr string) []exchange.FeeRatio {
	if len(ratiosStr) == 0 {
		return nil
	}

	ratios := strings.Split(ratiosStr, ",")
	rv := make([]exchange.FeeRatio, len(ratios))
	for i, r := range ratios {
		rv[i] = s.ratio(r)
	}
	return rv
}

// ratiosStrings converts the ratios into strings. It's because comparsions on sdk.Coin (or sdkmath.Int) are annoying.
func (s *TestSuite) ratiosStrings(ratios []exchange.FeeRatio) []string {
	return sliceStrings(ratios, exchange.FeeRatio.String)
}

// joinErrs joins the provided error strings into a single one to match what errors.Join does.
func (s *TestSuite) joinErrs(errs ...string) string {
	return strings.Join(errs, "\n")
}

// copyCoin creates a copy of a coin (as best as possible).
func (s *TestSuite) copyCoin(orig sdk.Coin) sdk.Coin {
	return sdk.NewCoin(orig.Denom, orig.Amount.AddRaw(0))
}

// copyCoinP copies a coin that's a reference.
func (s *TestSuite) copyCoinP(orig *sdk.Coin) *sdk.Coin {
	if orig == nil {
		return nil
	}
	rv := s.copyCoin(*orig)
	return &rv
}

// copyCoins creates a copy of coins (as best as possible).
func (s *TestSuite) copyCoins(orig []sdk.Coin) []sdk.Coin {
	return copySlice(orig, s.copyCoin)
}

// copyRatio creates a copy of a FeeRatio.
func (s *TestSuite) copyRatio(orig exchange.FeeRatio) exchange.FeeRatio {
	return exchange.FeeRatio{
		Price: s.copyCoin(orig.Price),
		Fee:   s.copyCoin(orig.Fee),
	}
}

// copyRatios creates a copy of a slice of FeeRatios.
func (s *TestSuite) copyRatios(orig []exchange.FeeRatio) []exchange.FeeRatio {
	return copySlice(orig, s.copyRatio)
}

// copyAccessGrant creates a copy of an AccessGrant.
func (s *TestSuite) copyAccessGrant(orig exchange.AccessGrant) exchange.AccessGrant {
	return exchange.AccessGrant{
		Address:     orig.Address,
		Permissions: copySlice(orig.Permissions, noOpCopier[exchange.Permission]),
	}
}

// copyAccessGrants creates a copy of a slice of AccessGrants.
func (s *TestSuite) copyAccessGrants(orig []exchange.AccessGrant) []exchange.AccessGrant {
	return copySlice(orig, s.copyAccessGrant)
}

// copyStrings creates a copy of a slice of strings.
func (s *TestSuite) copyStrings(orig []string) []string {
	return copySlice(orig, noOpCopier[string])
}

// copyMarket creates a deep copy of a market.
func (s *TestSuite) copyMarket(orig exchange.Market) exchange.Market {
	return exchange.Market{
		MarketId: orig.MarketId,
		MarketDetails: exchange.MarketDetails{
			Name:        orig.MarketDetails.Name,
			Description: orig.MarketDetails.Description,
			WebsiteUrl:  orig.MarketDetails.WebsiteUrl,
			IconUri:     orig.MarketDetails.IconUri,
		},
		FeeCreateAskFlat:          s.copyCoins(orig.FeeCreateAskFlat),
		FeeCreateBidFlat:          s.copyCoins(orig.FeeCreateBidFlat),
		FeeSellerSettlementFlat:   s.copyCoins(orig.FeeSellerSettlementFlat),
		FeeSellerSettlementRatios: s.copyRatios(orig.FeeSellerSettlementRatios),
		FeeBuyerSettlementFlat:    s.copyCoins(orig.FeeBuyerSettlementFlat),
		FeeBuyerSettlementRatios:  s.copyRatios(orig.FeeBuyerSettlementRatios),
		AcceptingOrders:           orig.AcceptingOrders,
		AllowUserSettlement:       orig.AllowUserSettlement,
		AccessGrants:              s.copyAccessGrants(orig.AccessGrants),
		ReqAttrCreateAsk:          s.copyStrings(orig.ReqAttrCreateAsk),
		ReqAttrCreateBid:          s.copyStrings(orig.ReqAttrCreateBid),
		AcceptingCommitments:      orig.AcceptingCommitments,
		FeeCreateCommitmentFlat:   s.copyCoins(orig.FeeCreateCommitmentFlat),
		CommitmentSettlementBips:  orig.CommitmentSettlementBips,
		IntermediaryDenom:         orig.IntermediaryDenom,
		ReqAttrCreateCommitment:   s.copyStrings(orig.ReqAttrCreateCommitment),
	}
}

// copyMarkets creates a copy of a slice of markets.
func (s *TestSuite) copyMarkets(orig []exchange.Market) []exchange.Market {
	return copySlice(orig, s.copyMarket)
}

// copyOrder creates a copy of an order.
func (s *TestSuite) copyOrder(orig exchange.Order) exchange.Order {
	rv := exchange.NewOrder(orig.OrderId)
	switch {
	case orig.IsAskOrder():
		rv.WithAsk(s.copyAskOrder(orig.GetAskOrder()))
	case orig.IsBidOrder():
		rv.WithBid(s.copyBidOrder(orig.GetBidOrder()))
	default:
		rv.Order = orig.Order
	}
	return *rv
}

// copyOrders creates a copy of a slice of orders.
func (s *TestSuite) copyOrders(orig []exchange.Order) []exchange.Order {
	return copySlice(orig, s.copyOrder)
}

// copyAskOrder creates a copy of an AskOrder.
func (s *TestSuite) copyAskOrder(orig *exchange.AskOrder) *exchange.AskOrder {
	if orig == nil {
		return nil
	}
	return &exchange.AskOrder{
		MarketId:                orig.MarketId,
		Seller:                  orig.Seller,
		Assets:                  s.copyCoin(orig.Assets),
		Price:                   s.copyCoin(orig.Price),
		SellerSettlementFlatFee: s.copyCoinP(orig.SellerSettlementFlatFee),
		AllowPartial:            orig.AllowPartial,
		ExternalId:              orig.ExternalId,
	}
}

// copyBidOrder creates a copy of a BidOrder.
func (s *TestSuite) copyBidOrder(orig *exchange.BidOrder) *exchange.BidOrder {
	if orig == nil {
		return nil
	}
	return &exchange.BidOrder{
		MarketId:            orig.MarketId,
		Buyer:               orig.Buyer,
		Assets:              s.copyCoin(orig.Assets),
		Price:               s.copyCoin(orig.Price),
		BuyerSettlementFees: s.copyCoins(orig.BuyerSettlementFees),
		AllowPartial:        orig.AllowPartial,
		ExternalId:          orig.ExternalId,
	}
}

// copyCommitment creates a copy of a commitment.
func (s *TestSuite) copyCommitment(orig exchange.Commitment) exchange.Commitment {
	return exchange.Commitment{
		Account:  orig.Account,
		MarketId: orig.MarketId,
		Amount:   s.copyCoins(orig.Amount),
	}
}

// copyCommitments creates a copy of a slice of commitments.
func (s *TestSuite) copyCommitments(orig []exchange.Commitment) []exchange.Commitment {
	return copySlice(orig, s.copyCommitment)
}

// copyPayment creates a copy of a payment.
func (s *TestSuite) copyPayment(orig exchange.Payment) exchange.Payment {
	return exchange.Payment{
		Source:       orig.Source,
		SourceAmount: s.copyCoins(orig.SourceAmount),
		Target:       orig.Target,
		TargetAmount: s.copyCoins(orig.TargetAmount),
		ExternalId:   orig.ExternalId,
	}
}

// copyPayments creates a coy of a slice of payments.
func (s *TestSuite) copyPayments(orig []exchange.Payment) []exchange.Payment {
	return copySlice(orig, s.copyPayment)
}

// untypeEvent applies sdk.TypedEventToEvent(tev) requiring it to not error.
func (s *TestSuite) untypeEvent(tev proto.Message) sdk.Event {
	rv, err := sdk.TypedEventToEvent(tev)
	s.Require().NoError(err, "TypedEventToEvent(%T)", tev)
	return rv
}

// untypeEvents applies sdk.TypedEventToEvent(tev) to each of the provided things, requiring it to not error.
func untypeEvents[P proto.Message](s *TestSuite, tevs []P) sdk.Events {
	rv := make(sdk.Events, len(tevs))
	for i, tev := range tevs {
		event, err := sdk.TypedEventToEvent(tev)
		s.Require().NoError(err, "[%d]TypedEventToEvent(%T)", i, tev)
		rv[i] = event
	}
	return rv
}

// creates a copy of a DenomSplit.
func (s *TestSuite) copyDenomSplit(orig exchange.DenomSplit) exchange.DenomSplit {
	return exchange.DenomSplit{
		Denom: orig.Denom,
		Split: orig.Split,
	}
}

// copyDenomSplits creates a copy of a slice of DenomSplits.
func (s *TestSuite) copyDenomSplits(orig []exchange.DenomSplit) []exchange.DenomSplit {
	return copySlice(orig, s.copyDenomSplit)
}

// copyParams creates a copy of exchange Params.
func (s *TestSuite) copyParams(orig *exchange.Params) *exchange.Params {
	if orig == nil {
		return nil
	}
	return &exchange.Params{
		DefaultSplit:         orig.DefaultSplit,
		DenomSplits:          s.copyDenomSplits(orig.DenomSplits),
		FeeCreatePaymentFlat: s.copyCoins(orig.FeeCreatePaymentFlat),
		FeeAcceptPaymentFlat: s.copyCoins(orig.FeeAcceptPaymentFlat),
	}
}

// copyGenState creates a copy of a GenesisState.
func (s *TestSuite) copyGenState(genState *exchange.GenesisState) *exchange.GenesisState {
	if genState == nil {
		return nil
	}
	return &exchange.GenesisState{
		Params:       s.copyParams(genState.Params),
		Markets:      s.copyMarkets(genState.Markets),
		Orders:       s.copyOrders(genState.Orders),
		LastMarketId: genState.LastMarketId,
		LastOrderId:  genState.LastOrderId,
		Commitments:  s.copyCommitments(genState.Commitments),
		Payments:     s.copyPayments(genState.Payments),
	}
}

// sortMarket sorts all the fields in a market.
func (s *TestSuite) sortMarket(market *exchange.Market) *exchange.Market {
	if len(market.FeeSellerSettlementRatios) > 0 {
		sort.Slice(market.FeeSellerSettlementRatios, func(i, j int) bool {
			if market.FeeSellerSettlementRatios[i].Price.Denom < market.FeeSellerSettlementRatios[j].Price.Denom {
				return true
			}
			if market.FeeSellerSettlementRatios[i].Price.Denom > market.FeeSellerSettlementRatios[j].Price.Denom {
				return false
			}
			return market.FeeSellerSettlementRatios[i].Fee.Denom < market.FeeSellerSettlementRatios[j].Fee.Denom
		})
	}
	if len(market.FeeBuyerSettlementRatios) > 0 {
		sort.Slice(market.FeeBuyerSettlementRatios, func(i, j int) bool {
			if market.FeeBuyerSettlementRatios[i].Price.Denom < market.FeeBuyerSettlementRatios[j].Price.Denom {
				return true
			}
			if market.FeeBuyerSettlementRatios[i].Price.Denom > market.FeeBuyerSettlementRatios[j].Price.Denom {
				return false
			}
			return market.FeeBuyerSettlementRatios[i].Fee.Denom < market.FeeBuyerSettlementRatios[j].Fee.Denom
		})
	}
	if len(market.AccessGrants) > 0 {
		sort.Slice(market.AccessGrants, func(i, j int) bool {
			// Horribly inefficient. Not meant for production.
			addrI, err := sdk.AccAddressFromBech32(market.AccessGrants[i].Address)
			s.Require().NoError(err, "AccAddressFromBech32(%q)", market.AccessGrants[i].Address)
			addrJ, err := sdk.AccAddressFromBech32(market.AccessGrants[j].Address)
			s.Require().NoError(err, "AccAddressFromBech32(%q)", market.AccessGrants[j].Address)
			return bytes.Compare(addrI, addrJ) < 0
		})
		for _, ag := range market.AccessGrants {
			sort.Slice(ag.Permissions, func(i, j int) bool {
				return ag.Permissions[i] < ag.Permissions[j]
			})
		}
	}
	return market
}

// sortGenState sorts the contents of a GenesisState.
func (s *TestSuite) sortGenState(genState *exchange.GenesisState) *exchange.GenesisState {
	if genState == nil {
		return nil
	}

	if genState.Params != nil && len(genState.Params.DenomSplits) > 0 {
		sort.Slice(genState.Params.DenomSplits, func(i, j int) bool {
			return genState.Params.DenomSplits[i].Denom < genState.Params.DenomSplits[j].Denom
		})
	}

	if len(genState.Markets) > 0 {
		sort.Slice(genState.Markets, func(i, j int) bool {
			return genState.Markets[i].MarketId < genState.Markets[j].MarketId
		})
		for _, market := range genState.Markets {
			s.sortMarket(&market)
		}
	}

	if len(genState.Orders) > 0 {
		sort.Slice(genState.Orders, func(i, j int) bool {
			return genState.Orders[i].OrderId < genState.Orders[j].OrderId
		})
	}

	if len(genState.Commitments) > 0 {
		sort.Slice(genState.Commitments, func(i, j int) bool {
			// compare market ids first
			if genState.Commitments[i].MarketId != genState.Commitments[j].MarketId {
				return genState.Commitments[i].MarketId < genState.Commitments[j].MarketId
			}
			// Then accounts. These need to be ordered by their byte representation.
			accd := s.compareAddrs(genState.Commitments[i].Account, genState.Commitments[j].Account)
			if accd != 0 {
				return accd < 0
			}
			// The market and account are the same. Since those are the only thing used in state
			// store keys of commitments, we don't compare the amounts. Just keep the existing ordering.
			return false
		})
	}

	if len(genState.Payments) > 0 {
		sort.Slice(genState.Payments, func(i, j int) bool {
			// Compare the sources first (using their byte representations).
			dSource := s.compareAddrs(genState.Payments[i].Source, genState.Payments[j].Source)
			if dSource != 0 {
				return dSource < 0
			}
			// Compare the external ids now.
			dEid := strings.Compare(genState.Payments[i].ExternalId, genState.Payments[j].ExternalId)
			if dEid != 0 {
				return dEid < 0
			}
			// Since the source and external id are the only things used in payment keys,
			// there's nothing else we should compare here, so just keep the existing ordering.
			return false
		})
	}

	return genState
}

// compareAddrs compares the two addresses in their byte representation.
// Returns -1 if addri < addrj, 0 if addri = addrj, and 1 if addri > addrj.
//
// This is an inefficient way of sorting and shouldn't be used outside unit tests.
// When used for sorting, the entries that are bech32 strings are sorted first,
// in byte order; then the non-bech32 strings follow in string order.
func (s *TestSuite) compareAddrs(addri, addrj string) int {
	if addri == addrj {
		return 0
	}

	acci, erri := sdk.AccAddressFromBech32(addri)
	accj, errj := sdk.AccAddressFromBech32(addrj)
	switch {
	case erri == nil && errj == nil:
		// They're both addresses, compare the bytes.
		return bytes.Compare(acci, accj)
	case erri == nil:
		// Only i is an actual address, so say addri < addrj
		return -1
	case errj == nil:
		// Only j is an actual address, so say addri > addrj
		return 1
	default:
		// Neither are actual addresses, so just compare the strings.
		if addri < addrj {
			return -1
		}
		return 1
	}
}

// getOrderIDStr gets a string of the given order's id.
func (s *TestSuite) getOrderIDStr(order *exchange.Order) string {
	if order == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%d", order.OrderId)
}

// getCommitmentString gets a simplified string for a commitment.
func (s *TestSuite) getCommitmentString(com exchange.Commitment) string {
	return fmt.Sprintf("%d: %s %s", com.MarketId, com.Account, com.Amount)
}

// getPaymentString subs in the name strings for the source and target
// and returns a Payment.String() of that.
func (s *TestSuite) getPaymentString(payment exchange.Payment) string {
	p2 := s.copyPayment(payment)
	p2.Source = s.getAddrStrName(p2.Source)
	p2.Target = s.getAddrStrName(p2.Target)
	return p2.String()
}

// getPaymentPString is like getPaymentString but takes in a pointer to a payment.
func (s *TestSuite) getPaymentPString(payment *exchange.Payment) string {
	if payment == nil {
		return "<nil>"
	}
	return s.getPaymentString(*payment)
}

// agCanOnly creates an AccessGrant for the given address with only the provided permission.
func (s *TestSuite) agCanOnly(addr sdk.AccAddress, perm exchange.Permission) exchange.AccessGrant {
	return exchange.AccessGrant{
		Address:     addr.String(),
		Permissions: []exchange.Permission{perm},
	}
}

// agCanAllBut creates an AccessGrant for the given address with all permissions except the provided one.
func (s *TestSuite) agCanAllBut(addr sdk.AccAddress, perm exchange.Permission) exchange.AccessGrant {
	rv := exchange.AccessGrant{
		Address: addr.String(),
	}
	for _, p := range exchange.AllPermissions() {
		if p != perm {
			rv.Permissions = append(rv.Permissions, p)
		}
	}
	return rv
}

// agCanEverything creates an AccessGrant for the given address with all permissions available.
func (s *TestSuite) agCanEverything(addr sdk.AccAddress) exchange.AccessGrant {
	return exchange.AccessGrant{
		Address:     addr.String(),
		Permissions: exchange.AllPermissions(),
	}
}

// addAddrLookup adds an entry to the addrLookupMap (for use in getAddrName).
func (s *TestSuite) addAddrLookup(addr sdk.AccAddress, name string) {
	s.addrLookupMap[string(addr)] = name
}

// getAddrName returns the name of the variable in this TestSuite holding the provided address.
func (s *TestSuite) getAddrName(addr sdk.AccAddress) string {
	if addr == nil {
		return "<nil>"
	}
	if addr.Empty() {
		return "<empty>"
	}
	if s.addrLookupMap != nil {
		rv, found := s.addrLookupMap[string(addr)]
		if found {
			return rv
		}
	}
	return addr.String()
}

// getAddrStrName returns the name of the variable in this TestSuite holding the provided address.
func (s *TestSuite) getAddrStrName(addrStr string) string {
	if addrStr == "" {
		return "<empty>"
	}
	addr, err := sdk.AccAddressFromBech32(addrStr)
	if err != nil {
		return addrStr
	}
	return s.getAddrName(addr)
}

// getStore gets the exchange store.
func (s *TestSuite) getStore() storetypes.KVStore {
	return s.k.GetStore(s.ctx)
}

// clearExchangeState deletes everything from the exchange state store.
func (s *TestSuite) clearExchangeState() {
	keeper.DeleteAll(s.getStore(), nil)
	s.accKeeper = nil
}

// stateEntryString converts the provided key and value into a "<key>"="<value>" string.
func (s *TestSuite) stateEntryString(key, value []byte) string {
	return fmt.Sprintf("%q=%q", key, value)
}

// dumpExchangeState creates a string for each entry in the hold state store.
// Each entry has the format `"<key>"="<value>"`.
func (s *TestSuite) dumpExchangeState() []string {
	var rv []string
	keeper.Iterate(s.getStore(), nil, func(key, value []byte) bool {
		rv = append(rv, s.stateEntryString(key, value))
		return false
	})
	return rv
}

// requireSetOrderInStore calls SetOrderInStore making sure it doesn't panic or return an error.
func (s *TestSuite) requireSetOrderInStore(store storetypes.KVStore, order *exchange.Order) {
	assertions.RequireNotPanicsNoErrorf(s.T(), func() error {
		return s.k.SetOrderInStore(store, *order)
	}, "SetOrderInStore(%d)", order.OrderId)
}

// requireSetOrdersInStore calls requireSetOrderInStore for each provided order and returns all the provided orders.
func (s *TestSuite) requireSetOrdersInStore(store storetypes.KVStore, orders ...*exchange.Order) []*exchange.Order {
	for _, order := range orders {
		s.requireSetOrderInStore(store, order)
	}
	return orders
}

// requireCreateMarket calls CreateMarket making sure it doesn't panic or return an error.
// It also uses the TestSuite.accKeeper for the market account.
func (s *TestSuite) requireCreateMarket(market exchange.Market) {
	if s.accKeeper == nil {
		s.accKeeper = NewMockAccountKeeper()
	}
	assertions.RequireNotPanicsNoErrorf(s.T(), func() error {
		_, err := s.k.WithAccountKeeper(s.accKeeper).CreateMarket(s.ctx, market)
		return err
	}, "CreateMarket(%d)", market.MarketId)
}

// requireCreateMarketUnmocked calls CreateMarket making sure it doesn't panic or return an error.
// This uses the normal account keeper (instead of a mocked one).
func (s *TestSuite) requireCreateMarketUnmocked(market exchange.Market) {
	assertions.RequireNotPanicsNoErrorf(s.T(), func() error {
		_, err := s.k.CreateMarket(s.ctx, market)
		return err
	}, "CreateMarket(%d)", market.MarketId)
}

// requireSetPaymentsInStore calls setPaymentInStore on each payment, making sure it doesn't panic or return an error.
func (s *TestSuite) requireSetPaymentsInStore(payments ...*exchange.Payment) {
	for i, payment := range payments {
		assertions.RequireNotPanicsNoErrorf(s.T(), func() error {
			return s.k.SetPaymentInStore(s.getStore(), payment)
		}, "[%d]: SetPaymentInStore(%s)", i, payment)
	}
}

// requireCreatePayments calls CreatePayment on each payment, making sure it doesn't panic or return an error.
func (s *TestSuite) requireCreatePayments(payments ...*exchange.Payment) {
	for i, payment := range payments {
		assertions.RequireNotPanicsNoErrorf(s.T(), func() error {
			return s.k.CreatePayment(s.ctx, payment)
		}, "[%d]: CreatePayment(%s)", i, payment)
	}
}

// assertAccAddressFromBech32 calls AccAddressFromBech32 asserting that it doesn't return an error.
func (s *TestSuite) assertAccAddressFromBech32(bech32 string, msg string, args ...interface{}) (sdk.AccAddress, bool) {
	rv, err := sdk.AccAddressFromBech32(bech32)
	ok := s.Assert().NoError(err, "AccAddressFromBech32(%q): "+msg, append([]interface{}{bech32}, args...))
	return rv, ok
}

// requireAccAddrFromBech32 calls AccAddressFromBech32 making sure it doesn't return an error.
// Panics can mess up other tests. That's why I'm using this instead of sdk.MustAccAddressFromBech32.
func (s *TestSuite) requireAccAddressFromBech32(bech32 string, msg string, args ...interface{}) sdk.AccAddress {
	rv, err := sdk.AccAddressFromBech32(bech32)
	s.Require().NoError(err, "AccAddressFromBech32(%q): "+msg, append([]interface{}{bech32}, args...))
	return rv
}

// assertEqualSlice asserts that expected = actual and returns true if so.
// If not, returns false and the stringer is applied to each entry and the comparison
// is redone on the strings in the hopes that it helps identify the problem.
// If the strings are also equal, each individual entry is compared.
func assertEqualSlice[T any](s *TestSuite, expected, actual []T, stringer func(T) string, msg string, args ...interface{}) bool {
	s.T().Helper()
	if s.Assert().Equalf(expected, actual, msg, args...) {
		return true
	}
	// compare each as strings in the hopes that makes it easier to identify the problem.
	expStrs := sliceStrings(expected, stringer)
	actStrs := sliceStrings(actual, stringer)
	if !s.Assert().Equalf(expStrs, actStrs, "strings: "+msg, args...) {
		return false
	}
	// They're the same as strings, so compare each individually.
	for i := range expected {
		s.Assert().Equalf(expected[i], actual[i], msg+fmt.Sprintf("[%d]", i), args...)
	}
	return false
}

// assertEqualOrderID asserts that two uint64 values are equal, and if not, includes their decimal form in the log.
// This is nice because .Equal failures output uints in hex, which can make it difficult to identify what's going on.
func (s *TestSuite) assertEqualOrderID(expected, actual uint64, msgAndArgs ...interface{}) bool {
	s.T().Helper()
	if s.Assert().Equal(expected, actual, msgAndArgs...) {
		return true
	}
	s.T().Logf("Expected order id: %d", expected)
	s.T().Logf("  Actual order id: %d", actual)
	return false
}

// assertEqualOrders asserts that the slices of orders are equal.
// If not, some further assertions are made to try to help try to clarify the differences.
func (s *TestSuite) assertEqualOrders(expected, actual []*exchange.Order, msg string, args ...interface{}) bool {
	s.T().Helper()
	return assertEqualSlice(s, expected, actual, s.getOrderIDStr, msg, args...)
}

// assertEqualCommitments asserts that the slices of commitments are equal.
// If not, some further assertions are made to try to help try to clarify the differences.
func (s *TestSuite) assertEqualCommitments(expected, actual []exchange.Commitment, msg string, args ...interface{}) bool {
	s.T().Helper()
	return assertEqualSlice(s, expected, actual, s.getCommitmentString, msg, args...)
}

// assertEqualPayment asserts that two payments are equal and helps identify exactly what's different if they're not.
// Returns true if they're equal, false otherwise.
func (s *TestSuite) assertEqualPayment(expected, actual *exchange.Payment, msg string, args ...interface{}) bool {
	s.T().Helper()
	if s.Assert().Equalf(expected, actual, msg, args...) {
		return true
	}

	// If either are nil, that'll be obvious in the above failure, no need to dig further.
	if expected == nil || actual == nil {
		return false
	}

	// compare them as strings for a possible easy way to identify the differences.
	eStr := s.getPaymentString(*expected)
	aStr := s.getPaymentString(*actual)
	if !s.Assert().Equalf(eStr, aStr, "as strings: "+msg, args...) {
		return false
	}

	// Check each field individually.
	s.Assert().Equalf(expected.Source, actual.Source, msg+" Source", args...)
	s.Assert().Equalf(expected.SourceAmount, actual.SourceAmount, msg+" SourceAmount", args...)
	s.Assert().Equalf(expected.Target, actual.Target, msg+" Target", args...)
	s.Assert().Equalf(expected.TargetAmount, actual.TargetAmount, msg+" TargetAmount", args...)
	s.Assert().Equalf(expected.ExternalId, actual.ExternalId, msg+" ExternalId", args...)
	return false
}

// assertEqualPayments asserts that two slices of payments are equal and helps identify exactly what's different if they're not.
// Returns true if they're equal, false otherwise.
func (s *TestSuite) assertEqualPayments(expected, actual []*exchange.Payment, msg string, args ...interface{}) bool {
	if s.Assert().Equalf(expected, actual, msg, args...) {
		return true
	}

	// compare each as strings in the hopes that makes it easier to identify the problem.
	expStrs := sliceStrings(expected, s.getPaymentPString)
	actStrs := sliceStrings(actual, s.getPaymentPString)
	if !s.Assert().Equalf(expStrs, actStrs, "strings: "+msg, args...) {
		return false
	}

	// They're the same as strings, compare each individually.
	args2 := make([]interface{}, len(args)+1)
	copy(args2, args)
	for i := range expected {
		args2[len(args2)-1] = i
		s.assertEqualPayment(expected[i], actual[i], msg+" [%d]", args2)
	}
	return false
}

// assertEqualCoins asserts that two Coins are equal and helps identify the differences if they're not.
// Returns true if they're equal, false otherwise.
func (s *TestSuite) assertEqualCoins(expected, actual sdk.Coins, msg string, args ...interface{}) bool {
	if s.Assert().Equalf(expected, actual, msg, args...) {
		return true
	}
	s.Assert().Equalf(s.coinsString(expected), s.coinsString(actual), msg+" (as strings)", args...)
	return false
}

// assertEqualNAVs asserts that two slices of NetAssetPrice are equal and helps identify the differences if they're not.
// Returns true if they're equal, false otherwise.
func (s *TestSuite) assertEqualNAVs(expected, actual []exchange.NetAssetPrice, msg string, args ...interface{}) bool {
	if s.Assert().Equalf(expected, actual, msg, args...) {
		return true
	}

	expStrs := sliceStrings(expected, exchange.NetAssetPrice.String)
	actStrs := sliceStrings(actual, exchange.NetAssetPrice.String)
	if !s.Assert().Equalf(expStrs, actStrs, msg+" (as strings)", args...) {
		return false
	}

	args2 := make([]interface{}, len(args)+1)
	copy(args2, args)
	for i := range expected {
		args2[len(args2)-1] = i
		s.assertEqualNAV(&expected[i], &actual[i], msg+" [%d]", args2...)
	}

	return false
}

// assertEqualNAV asserts that two NetAssetPrice entries are equal and helps identify the differences if they're not.
// Returns true if they're equal, false otherwise.
func (s *TestSuite) assertEqualNAV(expected, actual *exchange.NetAssetPrice, msg string, args ...interface{}) bool {
	if s.Assert().Equalf(expected, actual, msg, args...) {
		return true
	}

	if expected == nil || actual == nil {
		return false
	}

	if !s.Assert().Equalf(expected.String(), actual.String(), msg+" (as strings)", args...) {
		return false
	}

	s.Assert().Equalf(expected.Assets, actual.Assets, msg+" Assets", args...)
	s.Assert().Equalf(expected.Price, actual.Price, msg+" Price", args...)
	return false
}

// assertErrorValue is a wrapper for assertions.AssertErrorValue for this TestSuite.
func (s *TestSuite) assertErrorValue(theError error, expected string, msgAndArgs ...interface{}) bool {
	s.T().Helper()
	return assertions.AssertErrorValue(s.T(), theError, expected, msgAndArgs...)
}

// assertErrorContentsf is a wrapper for assertions.AssertErrorContentsf for this TestSuite.
func (s *TestSuite) assertErrorContentsf(theError error, contains []string, msg string, args ...interface{}) bool {
	s.T().Helper()
	return assertions.AssertErrorContentsf(s.T(), theError, contains, msg, args...)
}

// assertEqualEvents is a wrapper for assertions.AssertEqualEvents for this TestSuite.
func (s *TestSuite) assertEqualEvents(expected, actual sdk.Events, msgAndArgs ...interface{}) bool {
	s.T().Helper()
	return assertions.AssertEqualEvents(s.T(), expected, actual, msgAndArgs...)
}

// requirePanicEquals is a wrapper for assertions.RequirePanicEquals for this TestSuite.
func (s *TestSuite) requirePanicEquals(f assertions.PanicTestFunc, expected string, msgAndArgs ...interface{}) {
	s.T().Helper()
	assertions.RequirePanicEquals(s.T(), f, expected, msgAndArgs...)
}

// markerAddr gets the address of a marker account for the given denom.
func (s *TestSuite) markerAddr(denom string) sdk.AccAddress {
	markerAddr, err := markertypes.MarkerAddress(denom)
	s.Require().NoError(err, "MarkerAddress(%q)", denom)
	s.addAddrLookup(markerAddr, denom+"MarkerAddr")
	return markerAddr
}

// markerAccount returns a new marker account with the given supply.
func (s *TestSuite) markerAccount(supplyCoinStr string) markertypes.MarkerAccountI {
	supply := s.coin(supplyCoinStr)
	return &markertypes.MarkerAccount{
		BaseAccount: &authtypes.BaseAccount{Address: s.markerAddr(supply.Denom).String()},
		Status:      markertypes.StatusActive,
		Denom:       supply.Denom,
		Supply:      supply.Amount,
		MarkerType:  markertypes.MarkerType_RestrictedCoin,
		SupplyFixed: true,
	}
}

// markerNavSetEvent returns a new marke module EventSetNetAssetValue converted to sdk.Event.
func (s *TestSuite) markerNavSetEvent(assetsStr, priceStr string, marketID uint32) sdk.Event {
	assets := s.coin(assetsStr)
	event := &markertypes.EventSetNetAssetValue{
		Denom:  assets.Denom,
		Price:  priceStr,
		Volume: assets.Amount.String(),
		Source: fmt.Sprintf("x/exchange market %d", marketID),
	}
	return s.untypeEvent(event)
}

// metadataNavSetEvent returns a new metadata module EventSetNetAssetValue converted to sdk.Event.
func (s *TestSuite) metadataNavSetEvent(scopeID, priceStr string, marketID uint32) sdk.Event {
	event := &metadatatypes.EventSetNetAssetValue{
		ScopeId: scopeID,
		Price:   priceStr,
		Source:  fmt.Sprintf("x/exchange market %d", marketID),
	}
	return s.untypeEvent(event)
}

func (s *TestSuite) scopeID(base string) metadatatypes.MetadataAddress {
	s.T().Helper()
	s.Require().LessOrEqual(len(base), 16, "scopeID(%q): arg can only be 16 chars max")
	bz := []byte(base + "________________")[:16]
	uid, err := uuid.FromBytes(bz)
	s.Require().NoError(err, "uuid.FromBytes(%q)", string(bz))
	return metadatatypes.ScopeMetadataAddress(uid)
}
