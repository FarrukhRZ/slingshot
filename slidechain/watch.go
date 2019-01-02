package main

import (
	"context"
	"database/sql"

	"github.com/bobg/multichan"
	"github.com/chain/txvm/errors"
	"github.com/chain/txvm/protocol/bc"
	"github.com/chain/txvm/protocol/txvm"
	"github.com/interstellar/starlight/worizon"
	"github.com/stellar/go/xdr"
)

func watchPegs(db *sql.DB) func(worizon.Transaction) error {
	return func(tx worizon.Transaction) error {
		var env xdr.TransactionEnvelope
		err := xdr.SafeUnmarshalBase64(tx.EnvelopeXdr, &env)
		if err != nil {
			return errors.Wrap(err, "unmarshaling envelope XDR")
		}

		for _, op := range env.Tx.Operations {
			if op.Body.Type != xdr.OperationTypePayment {
				continue
			}
			payment := op.Body.PaymentOp
			if !payment.Destination.Equals(custAccountID) {
				continue
			}
			// TODO: this operation is a payment to the custodian's account - i.e., a peg.
			// Record it in the db and/or immediately issue imported funds on the sidechain.
		}
		return nil
	}
}

func watchExports(ctx context.Context, r *multichan.R) {
	for {
		got, ok := r.Read(ctx)
		if !ok {
			return
		}
		b := got.(*bc.Block)
		for _, tx := range b.Transactions {
			// Look for a retire-type ("X") entry followed by two log-type
			// ("L") entries, one specifying the Stellar asset code to peg
			// out and one specifying the Stellar recipient account ID.
			for i := 0; i < len(tx.Log)-3; i++ {
				item := tx.Log[i]
				if len(item) != 5 {
					continue
				}
				if item[0].(txvm.Bytes)[0] != txvm.RetireCode {
					continue
				}
				retiredAmount := int64(item[2].(txvm.Int))
				retiredAssetID := bc.HashFromBytes(item[3].(txvm.Bytes))

				stellarAssetCodeItem := tx.Log[i+1]
				if len(stellarAssetCodeItem) != 3 {
					continue
				}
				if stellarAssetCodeItem[0].(txvm.Bytes)[0] != txvm.LogCode {
					continue
				}
				var stellarAsset xdr.Asset
				err := xdr.SafeUnmarshal(stellarAssetCodeItem[2].(txvm.Bytes), &stellarAsset)
				if err != nil {
					continue
				}
				// TODO: check stellarAsset corresponds to retiredAssetID

				stellarRecipientItem := tx.Log[i+2]
				if len(stellarRecipientItem) != 3 {
					continue
				}
				if stellarRecipientItem[0].(txvm.Bytes)[0] != txvm.LogCode {
					continue
				}
				var stellarRecipient xdr.AccountId
				err = xdr.SafeUnmarshal(stellarRecipientItem[2].(txvm.Bytes), &stellarRecipient)
				if err != nil {
					continue
				}

				// TODO: This is an export operation.
				// Record it in the db and/or immediately peg-out funds on the main chain.

				i += 2
			}
		}
	}
}
