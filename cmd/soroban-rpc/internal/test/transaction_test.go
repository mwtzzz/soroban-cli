package test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"
	"github.com/stretchr/testify/assert"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellar/go/xdr"
	"github.com/stellar/soroban-tools/cmd/soroban-rpc/internal/methods"
)

func TestSendTransactionSucceedsWithoutResults(t *testing.T) {
	test := NewTest(t)

	ch := jhttp.NewChannel(test.server.URL, nil)
	client := jrpc2.NewClient(ch, nil)

	kp := keypair.Root(StandaloneNetworkPassphrase)
	address := kp.Address()
	account := txnbuild.NewSimpleAccount(address, 0)

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &account,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{
			&txnbuild.SetOptions{HomeDomain: txnbuild.NewHomeDomain("soroban.com")},
		},
		BaseFee: txnbuild.MinBaseFee,
		Preconditions: txnbuild.Preconditions{
			TimeBounds: txnbuild.NewInfiniteTimeout(),
		},
	})
	assert.NoError(t, err)
	tx, err = tx.Sign(StandaloneNetworkPassphrase, kp)
	assert.NoError(t, err)
	b64, err := tx.Base64()
	assert.NoError(t, err)

	request := methods.SendTransactionRequest{Transaction: b64}
	var result methods.SendTransactionResponse
	err = client.CallResult(context.Background(), "sendTransaction", request, &result)
	assert.NoError(t, err)

	expectedHashHex, err := tx.HashHex(StandaloneNetworkPassphrase)
	assert.NoError(t, err)

	assert.Equal(t, methods.SendTransactionResponse{
		ID:     expectedHashHex,
		Status: methods.TransactionPending,
	}, result)

	response := getTransactionStatus(t, client, expectedHashHex)
	assert.Equal(t, methods.TransactionSuccess, response.Status)
	assert.Equal(t, expectedHashHex, response.ID)
	assert.Nil(t, response.Error)
	assert.NotNil(t, response.EnvelopeXdr)
	assert.NotNil(t, response.ResultXdr)
	assert.NotNil(t, response.ResultMetaXdr)
	assert.Empty(t, response.Results)

	accountInfoRequest := methods.AccountRequest{
		Address: address,
	}
	var accountInfoResponse methods.AccountInfo
	err = client.CallResult(context.Background(), "getAccount", accountInfoRequest, &accountInfoResponse)
	assert.NoError(t, err)
	assert.Equal(t, methods.AccountInfo{ID: address, Sequence: 1}, accountInfoResponse)
}

func TestSendTransactionSucceedsWithResults(t *testing.T) {
	test := NewTest(t)

	ch := jhttp.NewChannel(test.server.URL, nil)
	client := jrpc2.NewClient(ch, nil)

	kp := keypair.Root(StandaloneNetworkPassphrase)
	address := kp.Address()
	account := txnbuild.NewSimpleAccount(address, 0)

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &account,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{
			createInstallContractCodeOperation(t, account.AccountID, testContract, true),
		},
		BaseFee: txnbuild.MinBaseFee,
		Preconditions: txnbuild.Preconditions{
			TimeBounds: txnbuild.NewInfiniteTimeout(),
		},
	})
	assert.NoError(t, err)
	tx, err = tx.Sign(StandaloneNetworkPassphrase, kp)
	assert.NoError(t, err)
	b64, err := tx.Base64()
	assert.NoError(t, err)

	request := methods.SendTransactionRequest{Transaction: b64}
	var result methods.SendTransactionResponse
	err = client.CallResult(context.Background(), "sendTransaction", request, &result)
	assert.NoError(t, err)

	expectedHashHex, err := tx.HashHex(StandaloneNetworkPassphrase)
	assert.NoError(t, err)

	assert.Equal(t, methods.SendTransactionResponse{
		ID:     expectedHashHex,
		Status: methods.TransactionPending,
	}, result)

	response := getTransactionStatus(t, client, expectedHashHex)
	assert.Equal(t, methods.TransactionSuccess, response.Status)
	assert.Equal(t, expectedHashHex, response.ID)
	assert.Nil(t, response.Error)

	// Check the result is what we expect
	assert.NotNil(t, response.EnvelopeXdr)
	assert.Equal(t, 1, len(response.Results))
	var resultVal xdr.ScVal
	assert.NoError(t, xdr.SafeUnmarshalBase64(response.Results[0].XDR, &resultVal))
	expectedContractID, err := hex.DecodeString("ea9fcb81ae54a29f6b3bf293847d3fd7e9a369fd1c80acafec6abd571317e0c2")
	assert.NoError(t, err)
	expectedObj := &xdr.ScObject{Type: xdr.ScObjectTypeScoBytes, Bin: &expectedContractID}
	expectedScVal := xdr.ScVal{Type: xdr.ScValTypeScvObject, Obj: &expectedObj}
	assert.True(t, expectedScVal.Equals(resultVal))

	expectedResult := xdr.TransactionResult{
		FeeCharged: 100,
		Result: xdr.TransactionResultResult{
			Code: xdr.TransactionResultCodeTxSuccess,
			Results: &[]xdr.OperationResult{
				{
					Code: xdr.OperationResultCodeOpInner,
					Tr: &xdr.OperationResultTr{
						Type: xdr.OperationTypeInvokeHostFunction,
						InvokeHostFunctionResult: &xdr.InvokeHostFunctionResult{
							Code:    xdr.InvokeHostFunctionResultCodeInvokeHostFunctionSuccess,
							Success: &expectedScVal,
						},
					},
				},
			},
		},
	}
	var resultXdr xdr.TransactionResult
	assert.NoError(t, xdr.SafeUnmarshalBase64(response.ResultXdr, &resultXdr))
	assert.Equal(t, expectedResult, resultXdr)

	var resultMetaXdr xdr.TransactionMeta
	assert.NoError(t, xdr.SafeUnmarshalBase64(response.ResultMetaXdr, &resultMetaXdr))

	// Check the txmeta is as expected
	resultMetaV3 := resultMetaXdr.MustV3()
	assert.Len(t, resultMetaV3.Operations, 1)
	assert.Len(t, *resultMetaV3.TxResult.Result.Results, 1)
	assert.True(
		t,
		(*resultMetaV3.TxResult.Result.Results)[0].Tr.MustInvokeHostFunctionResult().Success.Equals(expectedScVal),
	)

	accountInfoRequest := methods.AccountRequest{
		Address: address,
	}
	var accountInfoResponse methods.AccountInfo
	err = client.CallResult(context.Background(), "getAccount", accountInfoRequest, &accountInfoResponse)
	assert.NoError(t, err)
	assert.Equal(t, methods.AccountInfo{ID: address, Sequence: 1}, accountInfoResponse)
}

func getTransactionStatus(t *testing.T, client *jrpc2.Client, hash string) methods.TransactionStatusResponse {
	var result methods.TransactionStatusResponse
	for i := 0; i < 60; i++ {
		request := methods.GetTransactionStatusRequest{Hash: hash}
		err := client.CallResult(context.Background(), "getTransactionStatus", request, &result)
		assert.NoError(t, err)

		if result.Status == methods.TransactionPending {
			time.Sleep(time.Second)
			continue
		}

		return result
	}
	t.Fatal("getTransactionStatus timed out")
	return result
}

func TestSendTransactionBadSequence(t *testing.T) {
	test := NewTest(t)

	ch := jhttp.NewChannel(test.server.URL, nil)
	client := jrpc2.NewClient(ch, nil)

	kp := keypair.Root(StandaloneNetworkPassphrase)
	address := kp.Address()
	account := txnbuild.NewSimpleAccount(address, 0)

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount: &account,
		Operations: []txnbuild.Operation{
			&txnbuild.SetOptions{HomeDomain: txnbuild.NewHomeDomain("soroban.com")},
		},
		BaseFee: txnbuild.MinBaseFee,
		Preconditions: txnbuild.Preconditions{
			TimeBounds: txnbuild.NewInfiniteTimeout(),
		},
	})
	assert.NoError(t, err)
	tx, err = tx.Sign(StandaloneNetworkPassphrase, kp)
	assert.NoError(t, err)
	b64, err := tx.Base64()
	assert.NoError(t, err)

	request := methods.SendTransactionRequest{Transaction: b64}
	var result methods.SendTransactionResponse
	err = client.CallResult(context.Background(), "sendTransaction", request, &result)
	assert.NoError(t, err)

	expectedHashHex, err := tx.HashHex(StandaloneNetworkPassphrase)
	assert.NoError(t, err)

	assert.Equal(t, methods.SendTransactionResponse{
		ID:     expectedHashHex,
		Status: methods.TransactionPending,
	}, result)

	response := getTransactionStatus(t, client, expectedHashHex)
	assert.Equal(t, methods.TransactionError, response.Status)
	assert.Equal(t, expectedHashHex, response.ID)
	assert.Empty(t, response.Results)
	assert.Equal(t, "tx_submission_failed", response.Error.Code)
	assert.Equal(t, map[string]interface{}{
		"transaction": "tx_bad_seq",
	}, response.Error.Data["result_codes"])

	// assert that the transaction was not included in any ledger
	accountInfoRequest := methods.AccountRequest{
		Address: address,
	}
	var accountInfoResponse methods.AccountInfo
	err = client.CallResult(context.Background(), "getAccount", accountInfoRequest, &accountInfoResponse)
	assert.NoError(t, err)
	assert.Equal(t, methods.AccountInfo{ID: address, Sequence: 0}, accountInfoResponse)
}

func TestSendTransactionFailedInLedger(t *testing.T) {
	test := NewTest(t)

	ch := jhttp.NewChannel(test.server.URL, nil)
	client := jrpc2.NewClient(ch, nil)

	kp := keypair.Root(StandaloneNetworkPassphrase)
	address := kp.Address()
	account := txnbuild.NewSimpleAccount(address, 0)

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &account,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{
			// without the footprint the tx will fail
			createInstallContractCodeOperation(t, account.AccountID, testContract, false),
		},
		BaseFee: txnbuild.MinBaseFee,
		Preconditions: txnbuild.Preconditions{
			TimeBounds: txnbuild.NewInfiniteTimeout(),
		},
	})
	assert.NoError(t, err)
	tx, err = tx.Sign(StandaloneNetworkPassphrase, kp)
	assert.NoError(t, err)
	b64, err := tx.Base64()
	assert.NoError(t, err)

	request := methods.SendTransactionRequest{Transaction: b64}
	var result methods.SendTransactionResponse
	err = client.CallResult(context.Background(), "sendTransaction", request, &result)
	assert.NoError(t, err)

	expectedHashHex, err := tx.HashHex(StandaloneNetworkPassphrase)
	assert.NoError(t, err)

	assert.Equal(t, methods.SendTransactionResponse{
		ID:     expectedHashHex,
		Status: methods.TransactionPending,
	}, result)

	response := getTransactionStatus(t, client, expectedHashHex)
	assert.Equal(t, methods.TransactionError, response.Status)
	assert.Equal(t, expectedHashHex, response.ID)
	assert.Empty(t, response.Results)
	assert.Equal(t, "tx_failed", response.Error.Code)
	assert.Equal(t, "transaction included in ledger but failed", response.Error.Message)

	// assert that the transaction was not included in any ledger
	accountInfoRequest := methods.AccountRequest{
		Address: address,
	}
	var accountInfoResponse methods.AccountInfo
	err = client.CallResult(context.Background(), "getAccount", accountInfoRequest, &accountInfoResponse)
	assert.NoError(t, err)
	assert.Equal(t, methods.AccountInfo{ID: address, Sequence: 1}, accountInfoResponse)
}

func TestSendTransactionFailedInvalidXDR(t *testing.T) {
	test := NewTest(t)

	ch := jhttp.NewChannel(test.server.URL, nil)
	client := jrpc2.NewClient(ch, nil)

	request := methods.SendTransactionRequest{Transaction: "abcdef"}
	var response methods.SendTransactionResponse
	err := client.CallResult(context.Background(), "sendTransaction", request, &response)
	assert.NoError(t, err)

	assert.Equal(t, "", response.ID)
	assert.Equal(t, methods.TransactionError, response.Status)
	assert.Equal(t, "invalid_xdr", response.Error.Code)
	assert.Equal(t, "cannot unmarshal transaction: decoding EnvelopeType: decoding EnvelopeType: xdr:DecodeInt: unexpected EOF while decoding 4 bytes - read: '[105 183 29]'", response.Error.Message)
}
