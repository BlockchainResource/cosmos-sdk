package types

import (
	"bytes"
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/wire"
)

// Delegation represents the bond with tokens held by an account.  It is
// owned by one delegator, and is associated with the voting power of one
// pubKey.
type Delegation struct {
	DelegatorAddr sdk.AccAddress `json:"delegator_addr"`
	ValidatorAddr sdk.ValAddress `json:"validator_addr"`
	Shares        sdk.Dec        `json:"shares"`
	Height        int64          `json:"height"` // Last height bond updated
}

type delegationValue struct {
	Shares sdk.Dec
	Height int64
}

// defines a delegation with string value for the shares
type DelegationREST struct {
	DelegatorAddr string `json:"delegator_addr"`
	ValidatorAddr string `json:"validator_addr"`
	Shares        string `json:"shares"`
	Height        int64  `json:"height"`
}

// aggregates of all delegations, unbondings and redelegations
type DelegationSummary struct {
	Delegations          []DelegationREST      `json:"delegations"`
	UnbondingDelegations []UnbondingDelegation `json:"unbonding_delegations"`
	Redelegations        []RedelegationREST    `json:"redelegations"`
}

// return the delegation without fields contained within the key for the store
func MustMarshalDelegation(cdc *wire.Codec, delegation Delegation) []byte {
	val := delegationValue{
		delegation.Shares,
		delegation.Height,
	}
	return cdc.MustMarshalBinary(val)
}

// return the delegation without fields contained within the key for the store
func MustUnmarshalDelegation(cdc *wire.Codec, key, value []byte) Delegation {
	delegation, err := UnmarshalDelegation(cdc, key, value)
	if err != nil {
		panic(err)
	}
	return delegation
}

// return the delegation without fields contained within the key for the store
func UnmarshalDelegation(cdc *wire.Codec, key, value []byte) (delegation Delegation, err error) {
	var storeValue delegationValue
	err = cdc.UnmarshalBinary(value, &storeValue)
	if err != nil {
		err = fmt.Errorf("%v: %v", ErrNoDelegation(DefaultCodespace).Data(), err)
		return
	}

	addrs := key[1:] // remove prefix bytes
	if len(addrs) != 2*sdk.AddrLen {
		err = fmt.Errorf("%v", ErrBadDelegationAddr(DefaultCodespace).Data())
		return
	}

	delAddr := sdk.AccAddress(addrs[:sdk.AddrLen])
	valAddr := sdk.ValAddress(addrs[sdk.AddrLen:])

	return Delegation{
		DelegatorAddr: delAddr,
		ValidatorAddr: valAddr,
		Shares:        storeValue.Shares,
		Height:        storeValue.Height,
	}, nil
}

// nolint
func (d Delegation) Equal(d2 Delegation) bool {
	return bytes.Equal(d.DelegatorAddr, d2.DelegatorAddr) &&
		bytes.Equal(d.ValidatorAddr, d2.ValidatorAddr) &&
		d.Height == d2.Height &&
		d.Shares.Equal(d2.Shares)
}

// ensure fulfills the sdk validator types
var _ sdk.Delegation = Delegation{}

// nolint - for sdk.Delegation
func (d Delegation) GetDelegator() sdk.AccAddress { return d.DelegatorAddr }
func (d Delegation) GetValidator() sdk.ValAddress { return d.ValidatorAddr }
func (d Delegation) GetBondShares() sdk.Dec       { return d.Shares }

// HumanReadableString returns a human readable string representation of a
// Delegation. An error is returned if the Delegation's delegator or validator
// addresses cannot be Bech32 encoded.
func (d Delegation) HumanReadableString() (string, error) {
	resp := "Delegation \n"
	resp += fmt.Sprintf("Delegator: %s\n", d.DelegatorAddr)
	resp += fmt.Sprintf("Validator: %s\n", d.ValidatorAddr)
	resp += fmt.Sprintf("Shares: %s", d.Shares.String())
	resp += fmt.Sprintf("Height: %d", d.Height)

	return resp, nil
}

// changes delegation shares to string format
func (d Delegation) ToRest() DelegationREST {
	return DelegationREST{
		DelegatorAddr: d.DelegatorAddr.String(),
		ValidatorAddr: d.ValidatorAddr.String(),
		Height:        d.Height,
		Shares:        d.Shares.String(),
	}
}

// UnbondingDelegation reflects a delegation's passive unbonding queue.
type UnbondingDelegation struct {
	DelegatorAddr  sdk.AccAddress `json:"delegator_addr"`  // delegator
	ValidatorAddr  sdk.ValAddress `json:"validator_addr"`  // validator unbonding from operator addr
	CreationHeight int64          `json:"creation_height"` // height which the unbonding took place
	MinTime        time.Time      `json:"min_time"`        // unix time for unbonding completion
	InitialBalance sdk.Coin       `json:"initial_balance"` // atoms initially scheduled to receive at completion
	Balance        sdk.Coin       `json:"balance"`         // atoms to receive at completion
}

type ubdValue struct {
	CreationHeight int64
	MinTime        time.Time
	InitialBalance sdk.Coin
	Balance        sdk.Coin
}

// return the unbonding delegation without fields contained within the key for the store
func MustMarshalUBD(cdc *wire.Codec, ubd UnbondingDelegation) []byte {
	val := ubdValue{
		ubd.CreationHeight,
		ubd.MinTime,
		ubd.InitialBalance,
		ubd.Balance,
	}
	return cdc.MustMarshalBinary(val)
}

// unmarshal a unbonding delegation from a store key and value
func MustUnmarshalUBD(cdc *wire.Codec, key, value []byte) UnbondingDelegation {
	ubd, err := UnmarshalUBD(cdc, key, value)
	if err != nil {
		panic(err)
	}
	return ubd
}

// unmarshal a unbonding delegation from a store key and value
func UnmarshalUBD(cdc *wire.Codec, key, value []byte) (ubd UnbondingDelegation, err error) {
	var storeValue ubdValue
	err = cdc.UnmarshalBinary(value, &storeValue)
	if err != nil {
		return
	}

	addrs := key[1:] // remove prefix bytes
	if len(addrs) != 2*sdk.AddrLen {
		err = fmt.Errorf("%v", ErrBadDelegationAddr(DefaultCodespace).Data())
		return
	}
	delAddr := sdk.AccAddress(addrs[:sdk.AddrLen])
	valAddr := sdk.ValAddress(addrs[sdk.AddrLen:])

	return UnbondingDelegation{
		DelegatorAddr:  delAddr,
		ValidatorAddr:  valAddr,
		CreationHeight: storeValue.CreationHeight,
		MinTime:        storeValue.MinTime,
		InitialBalance: storeValue.InitialBalance,
		Balance:        storeValue.Balance,
	}, nil
}

// nolint
func (d UnbondingDelegation) Equal(d2 UnbondingDelegation) bool {
	bz1 := MsgCdc.MustMarshalBinary(&d)
	bz2 := MsgCdc.MustMarshalBinary(&d2)
	return bytes.Equal(bz1, bz2)
}

// HumanReadableString returns a human readable string representation of an
// UnbondingDelegation. An error is returned if the UnbondingDelegation's
// delegator or validator addresses cannot be Bech32 encoded.
func (d UnbondingDelegation) HumanReadableString() (string, error) {
	resp := "Unbonding Delegation \n"
	resp += fmt.Sprintf("Delegator: %s\n", d.DelegatorAddr)
	resp += fmt.Sprintf("Validator: %s\n", d.ValidatorAddr)
	resp += fmt.Sprintf("Creation height: %v\n", d.CreationHeight)
	resp += fmt.Sprintf("Min time to unbond (unix): %v\n", d.MinTime)
	resp += fmt.Sprintf("Expected balance: %s", d.Balance.String())

	return resp, nil

}

// Redelegation reflects a delegation's passive re-delegation queue.
type Redelegation struct {
	DelegatorAddr    sdk.AccAddress `json:"delegator_addr"`     // delegator
	ValidatorSrcAddr sdk.ValAddress `json:"validator_src_addr"` // validator redelegation source operator addr
	ValidatorDstAddr sdk.ValAddress `json:"validator_dst_addr"` // validator redelegation destination operator addr
	CreationHeight   int64          `json:"creation_height"`    // height which the redelegation took place
	MinTime          time.Time      `json:"min_time"`           // unix time for redelegation completion
	InitialBalance   sdk.Coin       `json:"initial_balance"`    // initial balance when redelegation started
	Balance          sdk.Coin       `json:"balance"`            // current balance
	SharesSrc        sdk.Dec        `json:"shares_src"`         // amount of source shares redelegating
	SharesDst        sdk.Dec        `json:"shares_dst"`         // amount of destination shares redelegating
}

type redValue struct {
	CreationHeight int64
	MinTime        time.Time
	InitialBalance sdk.Coin
	Balance        sdk.Coin
	SharesSrc      sdk.Dec
	SharesDst      sdk.Dec
}

// defines a redelegation with string value for the shares
type RedelegationREST struct {
	DelegatorAddr    string    `json:"delegator_addr"`     // delegator
	ValidatorSrcAddr string    `json:"validator_src_addr"` // validator redelegation source operator addr
	ValidatorDstAddr string    `json:"validator_dst_addr"` // validator redelegation destination operator addr
	CreationHeight   int64     `json:"creation_height"`    // height which the redelegation took place
	MinTime          time.Time `json:"min_time"`           // unix time for redelegation completion
	InitialBalance   sdk.Coin  `json:"initial_balance"`    // initial balance when redelegation started
	Balance          sdk.Coin  `json:"balance"`            // current balance
	SharesSrc        string    `json:"shares_src"`         // amount of source shares redelegating
	SharesDst        string    `json:"shares_dst"`         // amount of destination shares redelegating
}

// return the redelegation without fields contained within the key for the store
func MustMarshalRED(cdc *wire.Codec, red Redelegation) []byte {
	val := redValue{
		red.CreationHeight,
		red.MinTime,
		red.InitialBalance,
		red.Balance,
		red.SharesSrc,
		red.SharesDst,
	}
	return cdc.MustMarshalBinary(val)
}

// unmarshal a redelegation from a store key and value
func MustUnmarshalRED(cdc *wire.Codec, key, value []byte) Redelegation {
	red, err := UnmarshalRED(cdc, key, value)
	if err != nil {
		panic(err)
	}
	return red
}

// unmarshal a redelegation from a store key and value
func UnmarshalRED(cdc *wire.Codec, key, value []byte) (red Redelegation, err error) {
	var storeValue redValue
	err = cdc.UnmarshalBinary(value, &storeValue)
	if err != nil {
		return
	}

	addrs := key[1:] // remove prefix bytes
	if len(addrs) != 3*sdk.AddrLen {
		err = fmt.Errorf("%v", ErrBadRedelegationAddr(DefaultCodespace).Data())
		return
	}
	delAddr := sdk.AccAddress(addrs[:sdk.AddrLen])
	valSrcAddr := sdk.ValAddress(addrs[sdk.AddrLen : 2*sdk.AddrLen])
	valDstAddr := sdk.ValAddress(addrs[2*sdk.AddrLen:])

	return Redelegation{
		DelegatorAddr:    delAddr,
		ValidatorSrcAddr: valSrcAddr,
		ValidatorDstAddr: valDstAddr,
		CreationHeight:   storeValue.CreationHeight,
		MinTime:          storeValue.MinTime,
		InitialBalance:   storeValue.InitialBalance,
		Balance:          storeValue.Balance,
		SharesSrc:        storeValue.SharesSrc,
		SharesDst:        storeValue.SharesDst,
	}, nil
}

// nolint
func (d Redelegation) Equal(d2 Redelegation) bool {
	bz1 := MsgCdc.MustMarshalBinary(&d)
	bz2 := MsgCdc.MustMarshalBinary(&d2)
	return bytes.Equal(bz1, bz2)
}

// changes redelegation shares to string format
func (d Redelegation) ToRest() RedelegationREST {
	return RedelegationREST{
		DelegatorAddr:    d.DelegatorAddr.String(),
		ValidatorSrcAddr: d.ValidatorSrcAddr.String(),
		ValidatorDstAddr: d.ValidatorDstAddr.String(),
		CreationHeight:   d.CreationHeight,
		MinTime:          d.MinTime,
		InitialBalance:   d.InitialBalance,
		SharesSrc:        d.SharesSrc.String(),
		SharesDst:        d.SharesDst.String(),
	}
}

// HumanReadableString returns a human readable string representation of a
// Redelegation. An error is returned if the UnbondingDelegation's delegator or
// validator addresses cannot be Bech32 encoded.
func (d Redelegation) HumanReadableString() (string, error) {
	resp := "Redelegation \n"
	resp += fmt.Sprintf("Delegator: %s\n", d.DelegatorAddr)
	resp += fmt.Sprintf("Source Validator: %s\n", d.ValidatorSrcAddr)
	resp += fmt.Sprintf("Destination Validator: %s\n", d.ValidatorDstAddr)
	resp += fmt.Sprintf("Creation height: %v\n", d.CreationHeight)
	resp += fmt.Sprintf("Min time to unbond (unix): %v\n", d.MinTime)
	resp += fmt.Sprintf("Source shares: %s", d.SharesSrc.String())
	resp += fmt.Sprintf("Destination shares: %s", d.SharesDst.String())

	return resp, nil

}
