// Code generated by "stringer -type=TxnStateEnum"; DO NOT EDIT.

package sql

import "strconv"

const _TxnStateEnum_name = "NoTxnAutoRetryOpenAbortedRestartWaitCommitWait"

var _TxnStateEnum_index = [...]uint8{0, 5, 14, 18, 25, 36, 46}

func (i TxnStateEnum) String() string {
	if i < 0 || i >= TxnStateEnum(len(_TxnStateEnum_index)-1) {
		return "TxnStateEnum(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _TxnStateEnum_name[_TxnStateEnum_index[i]:_TxnStateEnum_index[i+1]]
}
