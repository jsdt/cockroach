// generated by stringer -type=Method; DO NOT EDIT

package proto

import "fmt"

const _Method_name = "ContainsGetPutConditionalPutIncrementDeleteDeleteRangeScanEndTransactionReapQueueEnqueueUpdateEnqueueMessageBatchAdminSplitAdminMergeInternalRangeLookupInternalHeartbeatTxnInternalGCInternalPushTxnInternalResolveIntentInternalResolveIntentRangeInternalMergeInternalTruncateLogInternalLeaderLeaseInternalBatch"

var _Method_index = [...]uint16{0, 8, 11, 14, 28, 37, 43, 54, 58, 72, 81, 94, 108, 113, 123, 133, 152, 172, 182, 197, 218, 244, 257, 276, 295, 308}

func (i Method) String() string {
	if i < 0 || i+1 >= Method(len(_Method_index)) {
		return fmt.Sprintf("Method(%d)", i)
	}
	return _Method_name[_Method_index[i]:_Method_index[i+1]]
}
