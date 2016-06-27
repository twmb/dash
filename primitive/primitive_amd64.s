// +build amd64
// +build !race

#define NOSPLIT 4

TEXT ·CompareAndSwapUintptr(SB), NOSPLIT, $0-33
	JMP ·CompareAndSwapUint64(SB)

TEXT ·CompareAndSwapInt64(SB), NOSPLIT, $0-33
	JMP ·CompareAndSwapUint64(SB)

TEXT ·CompareAndSwapUint64(SB), NOSPLIT, $0-33
	MOVQ     addr+0(FP), BP
	MOVQ     old+8(FP), AX
	MOVQ     new+16(FP), CX
	LOCK
	CMPXCHGQ CX, 0(BP)
	SETEQ    swapped+32(FP)
	JZ       swapped
	MOVQ     AX, fresh+24(FP)
	RET

swapped:
	MOVQ CX, fresh+24(FP)
	RET

TEXT ·CompareAndSwapInt32(SB), NOSPLIT, $0-21
	JMP ·CompareAndSwapUint32(SB)

TEXT ·CompareAndSwapUint32(SB), NOSPLIT, $0-21
	MOVQ     addr+0(FP), BP
	MOVL     old+8(FP), AX
	MOVL     new+12(FP), CX
	LOCK
	CMPXCHGL CX, 0(BP)
	SETEQ    swapped+20(FP)
	JZ       swapped
	MOVL     AX, fresh+16(FP)
	RET

swapped:
	MOVL CX, fresh+16(FP)
	RET
