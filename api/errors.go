package api

import (
	"context"

	"github.com/vocchain/go-vocchain/account"
	"github.com/vocchain/go-vocchain/asset"
	"github.com/vocchain/go-vocchain/blockchain/pseudohsm"
	"github.com/vocchain/go-vocchain/blockchain/rpc"
	"github.com/vocchain/go-vocchain/blockchain/signers"
	"github.com/vocchain/go-vocchain/blockchain/txbuilder"
	"github.com/vocchain/go-vocchain/errors"
	"github.com/vocchain/go-vocchain/net/http/httperror"
	"github.com/vocchain/go-vocchain/net/http/httpjson"
	"github.com/vocchain/go-vocchain/protocol/validation"
	"github.com/vocchain/go-vocchain/protocol/vm"
)

var (
	// ErrDefault is default Voc API Error
	ErrDefault = errors.New("Voc API Error")
)

func isTemporary(info httperror.Info, err error) bool {
	switch info.ChainCode {
	case "VOC00": // internal server error
		return true
	case "VOC01": // request timed out
		return true
	case "VOC61": // outputs currently reserved
		return true
	case "VOC06": // 1 or more action errors
		errs := errors.Data(err)["actions"].([]httperror.Response)
		temp := true
		for _, actionErr := range errs {
			temp = temp && isTemporary(actionErr.Info, nil)
		}
		return temp
	default:
		return false
	}
}

var respErrFormatter = map[error]httperror.Info{
	ErrDefault: {500, "VOC00", "Voc API Error"},

	// Signers error namespace (2xx)
	signers.ErrBadQuorum: {400, "VOC00", "Quorum must be greater than or equal to 1, and must be less than or equal to the length of xpubs"},
	signers.ErrBadXPub:   {400, "VOC01", "Invalid xpub format"},
	signers.ErrNoXPubs:   {400, "VOC02", "At least one xpub is required"},
	signers.ErrDupeXPub:  {400, "VOC03", "Root XPubs cannot contain the same key more than once"},

	// Contract error namespace (3xx)
	ErrCompileContract: {400, "VOC00", "Compile contract failed"},
	ErrInstContract:    {400, "VOC01", "Instantiate contract failed"},

	// Transaction error namespace (7xx)
	// Build transaction error namespace (70x ~ 72x)
	account.ErrInsufficient:         {400, "VOC00", "Funds of account are insufficient"},
	account.ErrImmature:             {400, "VOC01", "Available funds of account are immature"},
	account.ErrReserved:             {400, "VOC02", "Available UTXOs of account have been reserved"},
	account.ErrMatchUTXO:            {400, "VOC03", "UTXO with given hash not found"},
	ErrBadActionType:                {400, "VOC04", "Invalid action type"},
	ErrBadAction:                    {400, "VOC05", "Invalid action object"},
	ErrBadActionConstruction:        {400, "VOC06", "Invalid action construction"},
	txbuilder.ErrMissingFields:      {400, "VOC07", "One or more fields are missing"},
	txbuilder.ErrBadAmount:          {400, "VOC08", "Invalid asset amount"},
	account.ErrFindAccount:          {400, "VOC09", "Account not found"},
	asset.ErrFindAsset:              {400, "VOC10", "Asset not found"},
	txbuilder.ErrBadContractArgType: {400, "VOC11", "Invalid contract argument type"},
	txbuilder.ErrOrphanTx:           {400, "VOC12", "Transaction input UTXO not found"},
	txbuilder.ErrExtTxFee:           {400, "VOC713", "Transaction fee exceeded max limit"},
	txbuilder.ErrNoGasInput:         {400, "VOC714", "Transaction has no gas input"},

	// Submit transaction error namespace (73x ~ 79x)
	// Validation error (73x ~ 75x)
	validation.ErrTxVersion:                 {400, "VOC730", "Invalid transaction version"},
	validation.ErrWrongTransactionSize:      {400, "VOC731", "Invalid transaction size"},
	validation.ErrBadTimeRange:              {400, "VOC732", "Invalid transaction time range"},
	validation.ErrNotStandardTx:             {400, "VOC733", "Not standard transaction"},
	validation.ErrWrongCoinbaseTransaction:  {400, "VOC734", "Invalid coinbase transaction"},
	validation.ErrWrongCoinbaseAsset:        {400, "VOC735", "Invalid coinbase assetID"},
	validation.ErrCoinbaseArbitraryOversize: {400, "VOC736", "Invalid coinbase arbitrary size"},
	validation.ErrEmptyResults:              {400, "VOC737", "No results in the transaction"},
	validation.ErrMismatchedAssetID:         {400, "VOC738", "Mismatched assetID"},
	validation.ErrMismatchedPosition:        {400, "VOC739", "Mismatched value source/dest position"},
	validation.ErrMismatchedReference:       {400, "VOC740", "Mismatched reference"},
	validation.ErrMismatchedValue:           {400, "VOC741", "Mismatched value"},
	validation.ErrMissingField:              {400, "VOC742", "Missing required field"},
	validation.ErrNoSource:                  {400, "VOC743", "No source for value"},
	validation.ErrOverflow:                  {400, "VOC744", "Arithmetic overflow/underflow"},
	validation.ErrPosition:                  {400, "VOC745", "Invalid source or destination position"},
	validation.ErrUnbalanced:                {400, "VOC746", "Unbalanced asset amount between input and output"},
	validation.ErrOverGasCredit:             {400, "VOC747", "Gas credit has been spent"},
	validation.ErrGasCalculate:              {400, "VOC748", "Gas usage calculate got a math error"},

	// VM error (76x ~ 78x)
	vm.ErrAltStackUnderflow:  {400, "VOC760", "Alt stack underflow"},
	vm.ErrBadValue:           {400, "VOC761", "Bad value"},
	vm.ErrContext:            {400, "VOC762", "Wrong context"},
	vm.ErrDataStackUnderflow: {400, "VOC763", "Data stack underflow"},
	vm.ErrDisallowedOpcode:   {400, "VOC764", "Disallowed opcode"},
	vm.ErrDivZero:            {400, "VOC765", "Division by zero"},
	vm.ErrFalseVMResult:      {400, "VOC766", "False result for executing VM"},
	vm.ErrLongProgram:        {400, "VOC767", "Program size exceeds max int32"},
	vm.ErrRange:              {400, "VOC768", "Arithmetic range error"},
	vm.ErrReturn:             {400, "VOC769", "RETURN executed"},
	vm.ErrRunLimitExceeded:   {400, "VOC770", "Run limit exceeded because the VOC Fee is insufficient"},
	vm.ErrShortProgram:       {400, "VOC771", "Unexpected end of program"},
	vm.ErrToken:              {400, "VOC772", "Unrecognized token"},
	vm.ErrUnexpected:         {400, "VOC773", "Unexpected error"},
	vm.ErrUnsupportedVM:      {400, "VOC774", "Unsupported VM because the version of VM is mismatched"},
	vm.ErrVerifyFailed:       {400, "VOC775", "VERIFY failed"},

	// Mock HSM error namespace (8xx)
	pseudohsm.ErrDuplicateKeyAlias: {400, "VOC800", "Key Alias already exists"},
	pseudohsm.ErrLoadKey:           {400, "VOC801", "Key not found or wrong password"},
	pseudohsm.ErrDecrypt:           {400, "VOC802", "Could not decrypt key with given passphrase"},
}

// Map error values to standard voc error codes. Missing entries
// will map to internalErrInfo.
//
// TODO(jackson): Share one error table across Chain
// products/services so that errors are consistent.
var errorFormatter = httperror.Formatter{
	Default:     httperror.Info{500, "VOC000", "Voc API Error"},
	IsTemporary: isTemporary,
	Errors: map[error]httperror.Info{
		// General error namespace (0xx)
		context.DeadlineExceeded: {408, "VOC001", "Request timed out"},
		httpjson.ErrBadRequest:   {400, "VOC002", "Invalid request body"},
		rpc.ErrWrongNetwork:      {502, "VOC103", "A peer core is operating on a different blockchain network"},

		//accesstoken authz err namespace (86x)
		errNotAuthenticated: {401, "VOC860", "Request could not be authenticated"},
	},
}
