package app

import (
	"encoding/json"
	"io"
	"os"

	abci "github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/types"

	bam "github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/gov"
	"github.com/cosmos/cosmos-sdk/x/ibc"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/stake"
)

const (
	appName = "GaiaApp"
)

// default home directories for expected binaries
var (
	DefaultCLIHome  = os.ExpandEnv("$HOME/.gaiacli")
	DefaultNodeHome = os.ExpandEnv("$HOME/.gaiad")
)

// Extended ABCI application
type GaiaApp struct {
	*bam.BaseApp
	cdc *wire.Codec

	// keys to access the substores
	keyMain          *sdk.KVStoreKey
	keyAccount       *sdk.KVStoreKey
	keyIBC           *sdk.KVStoreKey
	keyStake         *sdk.KVStoreKey
	keySlashing      *sdk.KVStoreKey
	keyGov           *sdk.KVStoreKey
	keyFeeCollection *sdk.KVStoreKey
	keyParams        *sdk.KVStoreKey
	tkeyParams       *sdk.TransientStoreKey

	// Manage getting and setting accounts
	accountMapper       auth.AccountMapper
	feeCollectionKeeper auth.FeeCollectionKeeper
	bankKeeper          bank.Keeper
	ibcMapper           ibc.Mapper
	stakeKeeper         stake.Keeper
	slashingKeeper      slashing.Keeper
	govKeeper           gov.Keeper
	paramsKeeper        params.Keeper
}

// NewGaiaApp returns a reference to an initialized GaiaApp.
func NewGaiaApp(logger log.Logger, db dbm.DB, traceStore io.Writer, baseAppOptions ...func(*bam.BaseApp)) *GaiaApp {
	cdc := MakeCodec()

	bApp := bam.NewBaseApp(appName, logger, db, auth.DefaultTxDecoder(cdc), baseAppOptions...)
	bApp.SetCommitMultiStoreTracer(traceStore)

	var app = &GaiaApp{
		BaseApp:          bApp,
		cdc:              cdc,
		keyMain:          sdk.NewKVStoreKey("main"),
		keyAccount:       sdk.NewKVStoreKey("acc"),
		keyIBC:           sdk.NewKVStoreKey("ibc"),
		keyStake:         sdk.NewKVStoreKey("stake"),
		keySlashing:      sdk.NewKVStoreKey("slashing"),
		keyGov:           sdk.NewKVStoreKey("gov"),
		keyFeeCollection: sdk.NewKVStoreKey("fee"),
		keyParams:        sdk.NewKVStoreKey("params"),
		tkeyParams:       sdk.NewTransientStoreKey("transient_params"),
	}

	// define the accountMapper
	app.accountMapper = auth.NewAccountMapper(
		app.cdc,
		app.keyAccount,        // target store
		auth.ProtoBaseAccount, // prototype
	)

	// add handlers
	app.bankKeeper = bank.NewBaseKeeper(app.accountMapper)
	app.ibcMapper = ibc.NewMapper(app.cdc, app.keyIBC, app.RegisterCodespace(ibc.DefaultCodespace))

	app.paramsKeeper = params.NewKeeper(app.cdc, app.keyParams, app.tkeyParams)
	app.stakeKeeper = stake.NewKeeper(app.cdc, app.keyStake, app.bankKeeper, app.paramsKeeper.Subspace(stake.DefaultParamSpace), app.RegisterCodespace(stake.DefaultCodespace))
	app.slashingKeeper = slashing.NewKeeper(app.cdc, app.keySlashing, app.stakeKeeper, app.paramsKeeper.Subspace(slashing.DefaultParamSpace), app.RegisterCodespace(slashing.DefaultCodespace))
	app.stakeKeeper = app.stakeKeeper.WithValidatorHooks(app.slashingKeeper.ValidatorHooks())
	app.govKeeper = gov.NewKeeper(app.cdc, app.keyGov, app.paramsKeeper, app.paramsKeeper.Subspace(gov.DefaultParamSpace), app.bankKeeper, app.stakeKeeper, app.RegisterCodespace(gov.DefaultCodespace))
	app.feeCollectionKeeper = auth.NewFeeCollectionKeeper(app.cdc, app.keyFeeCollection)

	// register message routes
	app.Router().
		AddRoute("bank", bank.NewHandler(app.bankKeeper)).
		AddRoute("ibc", ibc.NewHandler(app.ibcMapper, app.bankKeeper)).
		AddRoute("stake", stake.NewHandler(app.stakeKeeper)).
		AddRoute("slashing", slashing.NewHandler(app.slashingKeeper)).
		AddRoute("gov", gov.NewHandler(app.govKeeper))

	app.QueryRouter().
		AddRoute("gov", gov.NewQuerier(app.govKeeper))

	// initialize BaseApp
	app.SetInitChainer(app.initChainer)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)
	app.SetAnteHandler(auth.NewAnteHandler(app.accountMapper, app.feeCollectionKeeper))
	app.MountStoresIAVL(app.keyMain, app.keyAccount, app.keyIBC, app.keyStake, app.keySlashing, app.keyGov, app.keyFeeCollection, app.keyParams)
	app.MountStore(app.tkeyParams, sdk.StoreTypeTransient)
	err := app.LoadLatestVersion(app.keyMain)
	if err != nil {
		cmn.Exit(err.Error())
	}

	return app
}

// custom tx codec
func MakeCodec() *wire.Codec {
	var cdc = wire.NewCodec()
	ibc.RegisterWire(cdc)
	bank.RegisterWire(cdc)
	stake.RegisterWire(cdc)
	slashing.RegisterWire(cdc)
	gov.RegisterWire(cdc)
	auth.RegisterWire(cdc)
	sdk.RegisterWire(cdc)
	wire.RegisterCrypto(cdc)
	return cdc
}

// application updates every end block
func (app *GaiaApp) BeginBlocker(ctx sdk.Context, req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	tags := slashing.BeginBlocker(ctx, req, app.slashingKeeper)

	return abci.ResponseBeginBlock{
		Tags: tags.ToKVPairs(),
	}
}

// application updates every end block
// nolint: unparam
func (app *GaiaApp) EndBlocker(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
	tags := gov.EndBlocker(ctx, app.govKeeper)
	validatorUpdates := stake.EndBlocker(ctx, app.stakeKeeper)
	// Add these new validators to the addr -> pubkey map.
	app.slashingKeeper.AddValidators(ctx, validatorUpdates)
	return abci.ResponseEndBlock{
		ValidatorUpdates: validatorUpdates,
		Tags:             tags,
	}
}

// custom logic for gaia initialization
func (app *GaiaApp) initChainer(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
	stateJSON := req.AppStateBytes
	// TODO is this now the whole genesis file?

	var genesisState GenesisState
	err := app.cdc.UnmarshalJSON(stateJSON, &genesisState)
	if err != nil {
		panic(err) // TODO https://github.com/cosmos/cosmos-sdk/issues/468
		// return sdk.ErrGenesisParse("").TraceCause(err, "")
	}

	// load the accounts
	for _, gacc := range genesisState.Accounts {
		acc := gacc.ToAccount()
		acc.AccountNumber = app.accountMapper.GetNextAccountNumber(ctx)
		app.accountMapper.SetAccount(ctx, acc)
	}

	// load the initial stake information
	validators, err := stake.InitGenesis(ctx, app.stakeKeeper, genesisState.StakeData)
	if err != nil {
		panic(err) // TODO https://github.com/cosmos/cosmos-sdk/issues/468
		// return sdk.ErrGenesisParse("").TraceCause(err, "")
	}

	// load the address to pubkey map
	slashing.InitGenesis(ctx, app.slashingKeeper, genesisState.SlashingData, genesisState.StakeData)

	gov.InitGenesis(ctx, app.govKeeper, genesisState.GovData)
	err = GaiaValidateGenesisState(genesisState)
	if err != nil {
		// TODO find a way to do this w/o panics
		panic(err)
	}

	err = slashing.InitGenesis(ctx, app.slashingKeeper, genesisState.SlashingData, genesisState.StakeData)
	if err != nil {
		panic(err)
	}

	return abci.ResponseInitChain{
		Validators: validators,
	}
}

// export the state of gaia for a genesis file
func (app *GaiaApp) ExportAppStateAndValidators() (appState json.RawMessage, validators []tmtypes.GenesisValidator, err error) {
	ctx := app.NewContext(true, abci.Header{})

	// iterate to get the accounts
	accounts := []GenesisAccount{}
	appendAccount := func(acc auth.Account) (stop bool) {
		account := NewGenesisAccountI(acc)
		accounts = append(accounts, account)
		return false
	}
	app.accountMapper.IterateAccounts(ctx, appendAccount)

	genState := GenesisState{
		Accounts:  accounts,
		StakeData: stake.WriteGenesis(ctx, app.stakeKeeper),
		GovData:   gov.WriteGenesis(ctx, app.govKeeper),
	}
	appState, err = wire.MarshalJSONIndent(app.cdc, genState)
	if err != nil {
		return nil, nil, err
	}
	validators = stake.WriteValidators(ctx, app.stakeKeeper)
	return appState, validators, nil
}
