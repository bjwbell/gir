package codegen

import (
	"bytes"
	"fmt"
	"math"
	"strings"

	"github.com/bjwbell/cmd/obj"
	"github.com/bjwbell/cmd/obj/x86"
	"github.com/bjwbell/ssa"
)

var lineno int32

var Maxarg int64

var hasdefer bool // flag that curfn has defer statement

var Debug_checknil int

// Smallest possible faulting page at address zero.
const minZeroPage = 4096

func Warn(fmt_ string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Printf("Warning: "+fmt_+"\n", args)
	} else {
		fmt.Printf("Warning: " + fmt_ + "\n")
	}
}

func Warnl(line int, fmt_ string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Printf("Warning (line %v): "+fmt_+" \n", line, args)
	} else {
		fmt.Printf("Warning (line %v): "+fmt_+" \n", line)
	}
}

// regnum returns the register (in cmd/internal/obj numbering) to
// which v has been allocated.  Panics if v is not assigned to a
// register.
// TODO: Make this panic again once it stops happening routinely.
func regnum(v *ssa.Value) int16 {
	reg := v.Block.Func.RegAlloc[v.ID]
	if reg == nil {
		v.Fatalf("nil regnum for value: %s\n%s\n", v.LongString(), v.Block.Func)
		return 0
	}
	return ssaRegToReg[reg.(*ssa.Register).Num()]
}

// autoVar returns a *Node and int64 representing the auto variable and offset within it
// where v should be spilled.
func autoVar(v *ssa.Value) (*Node, int64) {
	loc := v.Block.Func.RegAlloc[v.ID].(ssa.LocalSlot)
	return loc.N.(*Node), loc.Off
}

type LSym struct {
	Name      string
	Type      int16
	Version   int16
	Dupok     uint8
	Cfunc     uint8
	Nosplit   uint8
	Leaf      uint8
	Seenglobl uint8
	Onlist    uint8
	// Local means make the symbol local even when compiling Go code to reference Go
	// symbols in other shared libraries, as in this mode symbols are global by
	// default. "local" here means in the sense of the dynamic linker, i.e. not
	// visible outside of the module (shared library or executable) that contains its
	// definition. (When not compiling to support Go shared libraries, all symbols are
	// local in this sense unless there is a cgo_export_* directive).
	Local  bool
	Args   int32
	Locals int32
	Value  int64
	Size   int64
	/*Next   *LSym
	Gotype *LSym
	Autom  *Auto
	Text   *Prog
	Etext  *Prog
	Pcln   *Pcln
	P      []byte
	R      []Reloc*/

}

type Addr struct {
	Type   int16
	Reg    int16
	Index  int16
	Scale  int16 // Sometimes holds a register.
	Name   int8
	Class  int8
	Etype  uint8
	Offset int64
	Width  int64
	Sym    *LSym
	Gotype *LSym

	// argument value:
	//	for TYPE_SCONST, a string
	//	for TYPE_FCONST, a float64
	//	for TYPE_BRANCH, a *Prog (optional)
	//	for TYPE_TEXTSIZE, an int32 (optional)
	Val interface{}

	Node interface{} // for use by compiler

}

type Link struct {
	Goarm    int32
	Headtype int
	//Arch         *LinkArch
	Flag_shared  int32
	Flag_dynlink bool
	//Bso                *Biobuf
	Pathname           string
	Windows            int32
	Goroot             string
	Goroot_final       string
	Enforce_data_order int32
	//Hash               map[SymVer]*LSym
	//LineHist           LineHist
	Imports []string
	//Plist              *Plist
	//Plast              *Plist
	Sym_div    *LSym
	Sym_divu   *LSym
	Sym_mod    *LSym
	Sym_modu   *LSym
	Tlsg       *LSym
	Curp       *Prog
	Printp     *Prog
	Blitrl     *Prog
	Elitrl     *Prog
	Rexflag    int
	Rep        int
	Repn       int
	Lock       int
	Asmode     int
	Andptr     []byte
	And        [100]uint8
	Instoffset int64
	Autosize   int32
	Armsize    int32
	Pc         int64
	Tlsoffset  int
	//Diag               func(string, ...interface{})
	Mode    int
	Cursym  *LSym
	Version int
	Textp   *LSym
	Etextp  *LSym
}

type Prog struct {
	Ctxt   *Link
	Link   *Prog
	From   Addr
	From3  *Addr // optional
	To     Addr
	Opt    interface{}
	Forwd  *Prog
	Pcond  *Prog
	Rel    *Prog // Source of forward jumps on x86; pcrel on arm
	Pc     int64
	Lineno int32
	Spadj  int32
	As     int16
	Reg    int16
	RegTo2 int16 // 2nd register output operand
	Mark   uint16
	Optab  uint16
	Scond  uint8
	Back   uint8
	Ft     uint8
	Tt     uint8
	Isize  uint8
	Mode   int8

	//Info ProgInfo

}

// From3Type returns From3.Type, or TYPE_NONE when From3 is nil.
func (p *Prog) From3Type() int16 {
	if p.From3 == nil {
		return TYPE_NONE
	}
	return p.From3.Type
}

// From3Offset returns From3.Offset, or 0 when From3 is nil.
func (p *Prog) From3Offset() int64 {
	if p.From3 == nil {
		return 0
	}
	return p.From3.Offset
}

func (p *Prog) Line() string {
	return fmt.Sprintf("Line:%d", p.Lineno)
}

const (
	AXXX = 0 + iota
	ACALL
	ACHECKNIL
	ADATA
	ADUFFCOPY
	ADUFFZERO
	AEND
	AFUNCDATA
	AGLOBL
	AJMP
	ANOP
	APCDATA
	ARET
	ATEXT
	ATYPE
	AUNDEF
	AUSEFIELD
	AVARDEF
	AVARKILL
	A_ARCHSPECIFIC
)

const (
	ABase386 = (1 + iota) << 12
	ABaseARM
	ABaseAMD64
	ABasePPC64
	ABaseARM64
	AMask = 1<<12 - 1 // AND with this to use the opcode as an array index.
)

type opSet struct {
	lo    int
	names []string
}

// Not even worth sorting
var aSpace []opSet

func init() {
	RegisterRegister(x86.REG_AL, x86.REG_AL+len(x86.Register), x86.Rconv)
	RegisterOpcode(obj.ABaseAMD64, x86.Anames)
}

// RegisterOpcode binds a list of instruction names
// to a given instruction number range.
func RegisterOpcode(lo int, Anames []string) {
	aSpace = append(aSpace, opSet{lo, Anames})
}

func Aconv(a int) string {
	if a < A_ARCHSPECIFIC {
		return Anames[a]
	}
	for i := range aSpace {
		as := &aSpace[i]
		if as.lo <= a && a < as.lo+len(as.names) {
			return as.names[a-as.lo]
		}
	}
	return fmt.Sprintf("A???%d", a)
}

var Anames = []string{
	"XXX",
	"CALL",
	"CHECKNIL",
	"DATA",
	"DUFFCOPY",
	"DUFFZERO",
	"END",
	"FUNCDATA",
	"GLOBL",
	"JMP",
	"NOP",
	"PCDATA",
	"RET",
	"TEXT",
	"TYPE",
	"UNDEF",
	"USEFIELD",
	"VARDEF",
	"VARKILL",
}

func Bool2int(b bool) int {
	if b {
		return 1
	}
	return 0
}

func Dconv(p *Prog, a *Addr) string {
	var str string

	switch a.Type {
	default:
		str = fmt.Sprintf("type=%d", a.Type)

	case TYPE_NONE:
		str = ""
		if a.Name != NAME_NONE || a.Reg != 0 || a.Sym != nil {
			str = fmt.Sprintf("%v(%v)(NONE)", Mconv(a), Rconv(int(a.Reg)))
		}

	case TYPE_REG:
		// TODO(rsc): This special case is for x86 instructions like
		//	PINSRQ	CX,$1,X6
		// where the $1 is included in the p->to Addr.
		// Move into a new field.
		if a.Offset != 0 {
			str = fmt.Sprintf("$%d,%v", a.Offset, Rconv(int(a.Reg)))
			break
		}

		str = Rconv(int(a.Reg))
		if a.Name != TYPE_NONE || a.Sym != nil {
			str = fmt.Sprintf("%v(%v)(REG)", Mconv(a), Rconv(int(a.Reg)))
		}

	case TYPE_BRANCH:
		if a.Sym != nil {
			str = fmt.Sprintf("%s(SB)", a.Sym.Name)
		} else if p != nil && p.Pcond != nil {
			str = fmt.Sprint(p.Pcond.Pc)
		} else if a.Val != nil {
			str = fmt.Sprint(a.Val.(*Prog).Pc)
		} else {
			str = fmt.Sprintf("%d(PC)", a.Offset)
		}

	case TYPE_INDIR:
		str = fmt.Sprintf("*%s", Mconv(a))

	case TYPE_MEM:
		str = Mconv(a)
		if a.Index != REG_NONE {
			str += fmt.Sprintf("(%v*%d)", Rconv(int(a.Index)), int(a.Scale))
			panic("unexpected")
		}

	case TYPE_CONST:
		if a.Reg != 0 {
			str = fmt.Sprintf("$%v(%v)", Mconv(a), Rconv(int(a.Reg)))
		} else {
			str = fmt.Sprintf("$%v", Mconv(a))
		}

	case TYPE_TEXTSIZE:
		panic("unimplementedf")
		/*if a.Val.(int32) == ArgsSizeUnknown {
			str = fmt.Sprintf("$%d", a.Offset)
		} else {
			str = fmt.Sprintf("$%d-%d", a.Offset, a.Val.(int32))
		}*/

	case TYPE_FCONST:
		str = fmt.Sprintf("%.17g", a.Val.(float64))
		// Make sure 1 prints as 1.0
		if !strings.ContainsAny(str, ".e") {
			str += ".0"
		}
		str = fmt.Sprintf("$(%s)", str)

	case TYPE_SCONST:
		str = fmt.Sprintf("$%q", a.Val.(string))

	case TYPE_ADDR:
		str = fmt.Sprintf("$%s", Mconv(a))

	case TYPE_SHIFT:
		v := int(a.Offset)
		op := string("<<>>->@>"[((v>>5)&3)<<1:])
		if v&(1<<4) != 0 {
			str = fmt.Sprintf("R%d%c%cR%d", v&15, op[0], op[1], (v>>8)&15)
		} else {
			str = fmt.Sprintf("R%d%c%c%d", v&15, op[0], op[1], (v>>7)&31)
		}
		if a.Reg != 0 {
			str += fmt.Sprintf("(%v)", Rconv(int(a.Reg)))
		}

	case TYPE_REGREG:
		str = fmt.Sprintf("(%v, %v)", Rconv(int(a.Reg)), Rconv(int(a.Offset)))

	case TYPE_REGREG2:
		str = fmt.Sprintf("%v, %v", Rconv(int(a.Reg)), Rconv(int(a.Offset)))

	case TYPE_REGLIST:
		panic("unimplementedf")
		//str = regListConv(int(a.Offset))
	}

	return str
}

func Mconv(a *Addr) string {
	var str string

	switch a.Name {
	default:
		str = fmt.Sprintf("name=%d", a.Name)

	case NAME_NONE:
		switch {
		case a.Reg == REG_NONE:
			str = fmt.Sprint(a.Offset)
		case a.Offset == 0:
			str = fmt.Sprintf("(%v)", Rconv(int(a.Reg)))
		case a.Offset != 0:
			str = fmt.Sprintf("%d(%v)", a.Offset, Rconv(int(a.Reg)))
		}

	case NAME_EXTERN:
		str = fmt.Sprintf("%s%s(SB)", a.Sym.Name, offConv(a.Offset))

	case NAME_GOTREF:
		str = fmt.Sprintf("%s%s@GOT(SB)", a.Sym.Name, offConv(a.Offset))

	case NAME_STATIC:
		str = fmt.Sprintf("%s<>%s(SB)", a.Sym.Name, offConv(a.Offset))

	case NAME_AUTO:
		if a.Sym != nil {
			str = fmt.Sprintf("%s%s(SP)", a.Sym.Name, offConv(a.Offset))
		} else {
			str = fmt.Sprintf("%s(SP)", offConv(a.Offset))
		}

	case NAME_PARAM:
		if a.Sym != nil {
			str = fmt.Sprintf("%s%s(FP)", a.Sym.Name, offConv(a.Offset))
		} else {
			str = fmt.Sprintf("%s(FP)", offConv(a.Offset))
		}
	}
	return str
}

func offConv(off int64) string {
	if off == 0 {
		return ""
	}
	return fmt.Sprintf("%+d", off)
}

const REG_NONE = 0

type regSet struct {
	lo    int
	hi    int
	Rconv func(int) string
}

var regSpace []regSet

const (
	// Because of masking operations in the encodings, each register
	// space should start at 0 modulo some power of 2.
	RBase386   = 1 * 1024
	RBaseAMD64 = 2 * 1024
	RBaseARM   = 3 * 1024
	RBasePPC64 = 4 * 1024 // range [4k, 8k)
	RBaseARM64 = 8 * 1024 // range [8k, 12k)
)

// RegisterRegister binds a pretty-printer (Rconv) for register
// numbers to a given register number range.  Lo is inclusive,
// hi exclusive (valid registers are lo through hi-1).
func RegisterRegister(lo, hi int, Rconv func(int) string) {
	regSpace = append(regSpace, regSet{lo, hi, Rconv})
}

func Rconv(reg int) string {
	if reg == REG_NONE {
		return "NONE"
	}
	for i := range regSpace {
		rs := &regSpace[i]
		if rs.lo <= reg && reg < rs.hi {
			return rs.Rconv(reg)
		}
	}
	return fmt.Sprintf("R???%d", reg)
}

func (p *Prog) String() string {
	return p.Sprint(true)
}

func (p *Prog) Sprint(verbose bool) string {
	var buf bytes.Buffer
	if verbose {
		fmt.Fprintf(&buf, "%.5d (%v)\t%v", p.Pc, p.Line(), Aconv(int(p.As)))
	} else {
		fmt.Fprintf(&buf, "%s", Aconv(int(p.As)))
	}
	sep := "\t"
	if p.From.Type != TYPE_NONE {
		fmt.Fprintf(&buf, "%s%v", sep, Dconv(p, &p.From))
		sep = ", "
	}
	if p.Reg != REG_NONE {
		// Should not happen but might as well show it if it does
		fmt.Fprintf(&buf, "%s%v", sep, Rconv(int(p.Reg)))
		sep = ", "
	}
	if p.From3Type() != TYPE_NONE {
		if p.From3.Type == TYPE_CONST && (p.As == ADATA || p.As == ATEXT || p.As == AGLOBL) {
			// Special case - omit $.
			fmt.Fprintf(&buf, "%s%d", sep, p.From3.Offset)
		} else {
			fmt.Fprintf(&buf, "%s%v", sep, Dconv(p, p.From3))
		}
		sep = ", "
	}
	if p.To.Type != TYPE_NONE {
		fmt.Fprintf(&buf, "%s%v", sep, Dconv(p, &p.To))
	}
	if p.RegTo2 != REG_NONE {
		fmt.Fprintf(&buf, "%s%v", sep, Rconv(int(p.RegTo2)))
	}
	return buf.String()
}

// an unresolved branch
type branch struct {
	p *Prog      // branch instruction
	b *ssa.Block // target
}

type genState struct {
	// branches remembers all the branch instructions we've seen
	// and where they would like to go.
	branches []branch

	// bstart remembers where each block starts (indexed by block ID)
	bstart []*Prog

	// deferBranches remembers all the defer branches we've seen.
	deferBranches []*Prog

	// deferTarget remembers the (last) deferreturn call site.
	deferTarget *Prog
}

func Preamble() string {
	preamble := "// +build amd64 !noasm !appengine\n\n"
	preamble += "#include \"textflag.h\"\n\n"
	return preamble
}

func FuncProto(name string, frameSize, argsSize int) string {
	a := fmt.Sprintf("TEXT ·%v(SB),$%v-%v\n", name, frameSize, argsSize)
	return a
}

func Assemble(fn []*Prog) (assembly string) {
	assembly = ""
	for _, p := range fn {
		assembly += p.Sprint(false) + "\n"
	}
	return assembly
}

func GenGoProto(f *ssa.Func) (proto string, ok bool) {
	if f == nil {
		return "", false
	}

	proto = fmt.Sprintf("func %v()\n", f.Name)
	return proto, true
}

func GenAsm(f *ssa.Func) (asm string, ok bool) {
	if f == nil {
		return "", false
	}
	asm = FuncProto(f.Name, 0, 0)
	progs, success := GenProg(f)
	if !success {
		return "", false
	} else {
		for _, p := range progs {
			asm += p.Sprint(false) + "\n"
		}
	}
	return asm, true
}

func GenProg(f *ssa.Func) (fnProg []*Prog, ok bool) {

	Pc := new(Prog)

	var s genState

	// e := f.Config.Frontend().(*ssaExport)
	// We're about to emit a bunch of Progs.
	// Since the only way to get here is to explicitly request it,
	// just fail on unimplemented instead of trying to unwind our mess.
	// e.mustImplement = true

	// Remember where each block starts.
	s.bstart = make([]*Prog, f.NumBlocks())

	var valueProgs map[*Prog]*ssa.Value
	var blockProgs map[*Prog]*ssa.Block
	const logProgs = true
	if logProgs {
		valueProgs = make(map[*Prog]*ssa.Value, f.NumValues())
		blockProgs = make(map[*Prog]*ssa.Block, f.NumBlocks())
		f.Logf("genssa %s\n", f.Name)
		blockProgs[Pc] = f.Blocks[0]
	}
	var funcProgs []*Prog
	// Emit basic blocks
	for i, b := range f.Blocks {
		s.bstart[b.ID] = Pc
		// Emit values in block
		for _, v := range b.Values {
			//x := Pc
			progs := s.genValue(v)
			if logProgs {
				for _, prog := range progs {
					valueProgs[prog] = v
					funcProgs = append(funcProgs, prog)
				}
			}
		}
		// Emit control flow instructions for block
		var next *ssa.Block
		if i < len(f.Blocks)-1 {
			next = f.Blocks[i+1]
		}
		//x := Pc
		progs := s.genBlock(b, next)
		if logProgs {
			for _, prog := range progs {
				blockProgs[prog] = b
				funcProgs = append(funcProgs, prog)
			}
		}
	}

	// Resolve branches
	for _, br := range s.branches {
		br.p.To.Val = s.bstart[br.b.ID]
	}

	if s.deferBranches != nil && s.deferTarget == nil {
		panic("defer unsupported")
	}
	if len(s.deferBranches) > 0 {
		panic("defer unsupported")
	}

	if logProgs {
		for _, p := range funcProgs {
			var s string
			if v, ok := valueProgs[p]; ok {
				s = v.String()
			} else if b, ok := blockProgs[p]; ok {
				s = b.String()
			} else {
				s = "   " // most value and branch strings are 2-3 characters long
			}
			//f.Logf("%s\t%s\n", s, p)
			fmt.Println("ASM: ", s, "\t", p)
		}
	}

	// Allocate stack frame
	//allocauto(ptxt)

	// Generate gc bitmaps.
	/*liveness(Curfn, ptxt, gcargs, gclocals)
	gcsymdup(gcargs)
	gcsymdup(gclocals)*/

	// Add frame prologue.  Zero ambiguously live variables.
	/*Thearch.Defframe(ptxt)
	if Debug['f'] != 0 {
		frame(0)
	}*/

	// Remove leftover instrumentation from the instruction stream.
	//removevardef(ptxt)
	return funcProgs, true
}

// opregreg emits instructions for
//     dest := dest(To) op src(From)
// and also returns the created Prog so it
// may be further adjusted (offset, scale, etc).
/*func opregreg(op int, dest, src int16) *Prog {
	p := Prog(op)
	p.From.Type = obj.TYPE_REG
	p.To.Type = obj.TYPE_REG
	p.To.Reg = dest
	p.From.Reg = src
	return p
}*/

func NewProg() *Prog {
	p := new(Prog) // should be the only call to this; all others should use ctxt.NewProg
	//p.Ctxt = ctxt
	return p
}

func CreateProg(as int) *Prog {
	var p *Prog

	if as == obj.ADATA || as == obj.AGLOBL {
		Fatalf("already dumped data")

	} else {

		p = NewProg()
	}

	if lineno == 0 {
		Warn("prog: line num (0) not set")
	}

	p.As = int16(as)
	p.Lineno = lineno
	return p
}

func ProgAssembly(p *Prog) string {
	return ""
}

const (
	NAME_NONE = 0 + iota
	NAME_EXTERN
	NAME_STATIC
	NAME_AUTO
	NAME_PARAM
	// A reference to name@GOT(SB) is a reference to the entry in the global offset
	// table for 'name'.
	NAME_GOTREF
)

const (
	TYPE_NONE = 0
)

const (
	TYPE_BRANCH = 5 + iota
	TYPE_TEXTSIZE
	TYPE_MEM
	TYPE_CONST
	TYPE_FCONST
	TYPE_SCONST
	TYPE_REG
	TYPE_ADDR
	TYPE_SHIFT
	TYPE_REGREG
	TYPE_REGREG2
	TYPE_INDIR
	TYPE_REGLIST
)

// opregreg emits instructions for
//     dest := dest(To) op src(From)
// and also returns the created obj.Prog so it
// may be further adjusted (offset, scale, etc).
func opregreg(op int, dest, src int16) *Prog {
	p := CreateProg(op)
	p.From.Type = TYPE_REG
	p.To.Type = TYPE_REG
	p.To.Reg = dest
	p.From.Reg = src
	return p
}

func (s *genState) genValue(v *ssa.Value) []*Prog {
	var progs []*Prog
	var p *Prog
	switch v.Op {
	case ssa.OpAMD64ADDQ:
		// TODO: use addq instead of leaq if target is in the right register.
		p := CreateProg(x86.ALEAQ)
		p.From.Type = TYPE_MEM
		p.From.Reg = regnum(v.Args[0])
		p.From.Scale = 1
		p.From.Index = regnum(v.Args[1])
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	case ssa.OpAMD64ADDL:
		p = CreateProg(x86.ALEAL)
		p.From.Type = TYPE_MEM
		p.From.Reg = regnum(v.Args[0])
		p.From.Scale = 1
		p.From.Index = regnum(v.Args[1])
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	// 2-address opcode arithmetic, symmetric
	case ssa.OpAMD64ADDSS, ssa.OpAMD64ADDSD,
		ssa.OpAMD64ANDQ, ssa.OpAMD64ANDL,
		ssa.OpAMD64ORQ, ssa.OpAMD64ORL,
		ssa.OpAMD64XORQ, ssa.OpAMD64XORL,
		ssa.OpAMD64MULQ, ssa.OpAMD64MULL,
		ssa.OpAMD64MULSS, ssa.OpAMD64MULSD, ssa.OpAMD64PXOR:
		r := regnum(v)
		x := regnum(v.Args[0])
		y := regnum(v.Args[1])
		if x != r && y != r {
			opregreg(regMoveByTypeAMD64(v.Type), r, x)
			x = r
		}
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_REG
		p.To.Type = TYPE_REG
		p.To.Reg = r
		if x == r {
			p.From.Reg = y
		} else {
			p.From.Reg = x
		}
		progs = append(progs, p)
	// 2-address opcode arithmetic, not symmetric
	case ssa.OpAMD64SUBQ, ssa.OpAMD64SUBL:
		r := regnum(v)
		x := regnum(v.Args[0])
		y := regnum(v.Args[1])
		var neg bool
		if y == r {
			// compute -(y-x) instead
			x, y = y, x
			neg = true
		}
		if x != r {
			opregreg(regMoveByTypeAMD64(v.Type), r, x)
		}
		opregreg(int(v.Op.Asm()), r, y)

		if neg {
			p = CreateProg(x86.ANEGQ) // TODO: use correct size?  This is mostly a hack until regalloc does 2-address correctly
			p.To.Type = TYPE_REG
			p.To.Reg = r
		}
		progs = append(progs, p)
	case ssa.OpAMD64SUBSS, ssa.OpAMD64SUBSD, ssa.OpAMD64DIVSS, ssa.OpAMD64DIVSD:
		r := regnum(v)
		x := regnum(v.Args[0])
		y := regnum(v.Args[1])
		if y == r && x != r {
			// r/y := x op r/y, need to preserve x and rewrite to
			// r/y := r/y op x15
			x15 := int16(x86.REG_X15)
			// register move y to x15
			// register move x to y
			// rename y with x15
			opregreg(regMoveByTypeAMD64(v.Type), x15, y)
			opregreg(regMoveByTypeAMD64(v.Type), r, x)
			y = x15
		} else if x != r {
			opregreg(regMoveByTypeAMD64(v.Type), r, x)
		}
		opregreg(int(v.Op.Asm()), r, y)

	case ssa.OpAMD64DIVQ, ssa.OpAMD64DIVL, ssa.OpAMD64DIVW,
		ssa.OpAMD64DIVQU, ssa.OpAMD64DIVLU, ssa.OpAMD64DIVWU:

		// Arg[0] is already in AX as it's the only register we allow
		// and AX is the only output
		x := regnum(v.Args[1])

		// CPU faults upon signed overflow, which occurs when most
		// negative int is divided by -1.
		var j *Prog
		if v.Op == ssa.OpAMD64DIVQ || v.Op == ssa.OpAMD64DIVL ||
			v.Op == ssa.OpAMD64DIVW {

			var c *Prog
			switch v.Op {
			case ssa.OpAMD64DIVQ:
				c = CreateProg(x86.ACMPQ)
				j = CreateProg(x86.AJEQ)
				// go ahead and sign extend to save doing it later
				CreateProg(x86.ACQO)

			case ssa.OpAMD64DIVL:
				c = CreateProg(x86.ACMPL)
				j = CreateProg(x86.AJEQ)
				CreateProg(x86.ACDQ)

			case ssa.OpAMD64DIVW:
				c = CreateProg(x86.ACMPW)
				j = CreateProg(x86.AJEQ)
				CreateProg(x86.ACWD)
			}
			c.From.Type = TYPE_REG
			c.From.Reg = x
			c.To.Type = TYPE_CONST
			c.To.Offset = -1

			j.To.Type = TYPE_BRANCH

		}

		// for unsigned ints, we sign extend by setting DX = 0
		// signed ints were sign extended above
		if v.Op == ssa.OpAMD64DIVQU ||
			v.Op == ssa.OpAMD64DIVLU ||
			v.Op == ssa.OpAMD64DIVWU {
			c := CreateProg(x86.AXORQ)
			c.From.Type = TYPE_REG
			c.From.Reg = x86.REG_DX
			c.To.Type = TYPE_REG
			c.To.Reg = x86.REG_DX
		}

		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_REG
		p.From.Reg = x

		// signed division, rest of the check for -1 case
		if j != nil {
			j2 := CreateProg(obj.AJMP)
			j2.To.Type = TYPE_BRANCH

			var n *Prog
			if v.Op == ssa.OpAMD64DIVQ || v.Op == ssa.OpAMD64DIVL ||
				v.Op == ssa.OpAMD64DIVW {
				// n * -1 = -n
				n = CreateProg(x86.ANEGQ)
				n.To.Type = TYPE_REG
				n.To.Reg = x86.REG_AX
			} else {
				// n % -1 == 0
				n = CreateProg(x86.AXORQ)
				n.From.Type = TYPE_REG
				n.From.Reg = x86.REG_DX
				n.To.Type = TYPE_REG
				n.To.Reg = x86.REG_DX
			}

			j.To.Val = n
			panic("TODO")
			//j2.To.Val = Pc
		}
		progs = append(progs, p)
	case ssa.OpAMD64HMULL, ssa.OpAMD64HMULW, ssa.OpAMD64HMULB,
		ssa.OpAMD64HMULLU, ssa.OpAMD64HMULWU, ssa.OpAMD64HMULBU:
		// the frontend rewrites constant division by 8/16/32 bit integers into
		// HMUL by a constant

		// Arg[0] is already in AX as it's the only register we allow
		// and DX is the only output we care about (the high bits)
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_REG
		p.From.Reg = regnum(v.Args[1])

		// IMULB puts the high portion in AH instead of DL,
		// so move it to DL for consistency
		if v.Type.Size() == 1 {
			m := CreateProg(x86.AMOVB)
			m.From.Type = TYPE_REG
			m.From.Reg = x86.REG_AH
			m.To.Type = TYPE_REG
			m.To.Reg = x86.REG_DX
		}
		progs = append(progs, p)
	case ssa.OpAMD64SHLQ, ssa.OpAMD64SHLL,
		ssa.OpAMD64SHRQ, ssa.OpAMD64SHRL,
		ssa.OpAMD64SARQ, ssa.OpAMD64SARL:
		x := regnum(v.Args[0])
		r := regnum(v)
		if x != r {
			if r == x86.REG_CX {
				v.Fatalf("can't implement %s, target and shift both in CX", v.LongString())
			}
			p = CreateProg(regMoveAMD64(v.Type.Size()))
			p.From.Type = TYPE_REG
			p.From.Reg = x
			p.To.Type = TYPE_REG
			p.To.Reg = r
		}
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_REG
		p.From.Reg = regnum(v.Args[1]) // should be CX
		p.To.Type = TYPE_REG
		p.To.Reg = r
		progs = append(progs, p)
	case ssa.OpAMD64ADDQconst, ssa.OpAMD64ADDLconst:
		// TODO: use addq instead of leaq if target is in the right register.
		var asm int
		switch v.Op {
		case ssa.OpAMD64ADDQconst:
			asm = x86.ALEAQ
		case ssa.OpAMD64ADDLconst:
			asm = x86.ALEAL
		}
		p = CreateProg(asm)
		p.From.Type = TYPE_MEM
		p.From.Reg = regnum(v.Args[0])
		p.From.Offset = v.AuxInt
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	case ssa.OpAMD64MULQconst, ssa.OpAMD64MULLconst:
		r := regnum(v)
		x := regnum(v.Args[0])
		if r != x {
			p = CreateProg(regMoveAMD64(v.Type.Size()))
			p.From.Type = TYPE_REG
			p.From.Reg = x
			p.To.Type = TYPE_REG
			p.To.Reg = r
		}
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_CONST
		p.From.Offset = v.AuxInt
		p.To.Type = TYPE_REG
		p.To.Reg = r
		// TODO: Teach doasm to compile the three-address multiply imul $c, r1, r2
		// instead of using the MOVQ above.
		//p.From3 = new(obj.Addr)
		//p.From3.Type = TYPE_REG
		//p.From3.Reg = regnum(v.Args[0])
		progs = append(progs, p)
	case
		ssa.OpAMD64ANDQconst, ssa.OpAMD64ANDLconst,
		ssa.OpAMD64ORQconst, ssa.OpAMD64ORLconst,
		ssa.OpAMD64XORQconst, ssa.OpAMD64XORLconst,
		ssa.OpAMD64SUBQconst, ssa.OpAMD64SUBLconst,
		ssa.OpAMD64SHLQconst, ssa.OpAMD64SHLLconst,
		ssa.OpAMD64SHRQconst, ssa.OpAMD64SHRLconst,
		ssa.OpAMD64SARQconst, ssa.OpAMD64SARLconst,
		ssa.OpAMD64ROLQconst, ssa.OpAMD64ROLLconst:
		// This code compensates for the fact that the register allocator
		// doesn't understand 2-address instructions yet.  TODO: fix that.
		x := regnum(v.Args[0])
		r := regnum(v)
		if x != r {
			p = CreateProg(regMoveAMD64(v.Type.Size()))
			p.From.Type = TYPE_REG
			p.From.Reg = x
			p.To.Type = TYPE_REG
			p.To.Reg = r
		}
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_CONST
		p.From.Offset = v.AuxInt
		p.To.Type = TYPE_REG
		p.To.Reg = r
		progs = append(progs, p)
	case ssa.OpAMD64SBBQcarrymask, ssa.OpAMD64SBBLcarrymask:
		r := regnum(v)
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_REG
		p.From.Reg = r
		p.To.Type = TYPE_REG
		p.To.Reg = r
		progs = append(progs, p)
	case ssa.OpAMD64LEAQ1, ssa.OpAMD64LEAQ2, ssa.OpAMD64LEAQ4, ssa.OpAMD64LEAQ8:
		p = CreateProg(x86.ALEAQ)
		p.From.Type = TYPE_MEM
		p.From.Reg = regnum(v.Args[0])
		switch v.Op {
		case ssa.OpAMD64LEAQ1:
			p.From.Scale = 1
		case ssa.OpAMD64LEAQ2:
			p.From.Scale = 2
		case ssa.OpAMD64LEAQ4:
			p.From.Scale = 4
		case ssa.OpAMD64LEAQ8:
			p.From.Scale = 8
		}
		p.From.Index = regnum(v.Args[1])
		addAux(&p.From, v)
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	case ssa.OpAMD64LEAQ:
		p = CreateProg(x86.ALEAQ)
		p.From.Type = TYPE_MEM
		p.From.Reg = regnum(v.Args[0])
		addAux(&p.From, v)
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	case ssa.OpAMD64CMPQ, ssa.OpAMD64CMPL, ssa.OpAMD64CMPW, ssa.OpAMD64CMPB,
		ssa.OpAMD64TESTQ, ssa.OpAMD64TESTL, ssa.OpAMD64TESTW, ssa.OpAMD64TESTB:
		opregreg(int(v.Op.Asm()), regnum(v.Args[1]), regnum(v.Args[0]))
	case ssa.OpAMD64UCOMISS, ssa.OpAMD64UCOMISD:
		// Go assembler has swapped operands for UCOMISx relative to CMP,
		// must account for that right here.
		opregreg(int(v.Op.Asm()), regnum(v.Args[0]), regnum(v.Args[1]))
	case ssa.OpAMD64CMPQconst, ssa.OpAMD64CMPLconst, ssa.OpAMD64CMPWconst, ssa.OpAMD64CMPBconst,
		ssa.OpAMD64TESTQconst, ssa.OpAMD64TESTLconst, ssa.OpAMD64TESTWconst, ssa.OpAMD64TESTBconst:
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_REG
		p.From.Reg = regnum(v.Args[0])
		p.To.Type = TYPE_CONST
		p.To.Offset = v.AuxInt
		progs = append(progs, p)
	case ssa.OpAMD64MOVLconst, ssa.OpAMD64MOVQconst:
		x := regnum(v)
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_CONST
		var i int64
		switch v.Op {
		case ssa.OpAMD64MOVLconst:
			i = int64(int32(v.AuxInt))
		case ssa.OpAMD64MOVQconst:
			i = v.AuxInt
		}
		p.From.Offset = i
		p.To.Type = TYPE_REG
		p.To.Reg = x
		progs = append(progs, p)
	case ssa.OpAMD64MOVSSconst, ssa.OpAMD64MOVSDconst:
		x := regnum(v)
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_FCONST
		p.From.Val = math.Float64frombits(uint64(v.AuxInt))
		p.To.Type = TYPE_REG
		p.To.Reg = x
		progs = append(progs, p)
	case ssa.OpAMD64MOVQload, ssa.OpAMD64MOVSSload, ssa.OpAMD64MOVSDload, ssa.OpAMD64MOVLload, ssa.OpAMD64MOVWload, ssa.OpAMD64MOVBload, ssa.OpAMD64MOVBQSXload, ssa.OpAMD64MOVOload:
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_MEM
		p.From.Reg = regnum(v.Args[0])
		addAux(&p.From, v)
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	case ssa.OpAMD64MOVQloadidx8, ssa.OpAMD64MOVSDloadidx8:
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_MEM
		p.From.Reg = regnum(v.Args[0])
		addAux(&p.From, v)
		p.From.Scale = 8
		p.From.Index = regnum(v.Args[1])
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	case ssa.OpAMD64MOVSSloadidx4:
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_MEM
		p.From.Reg = regnum(v.Args[0])
		addAux(&p.From, v)
		p.From.Scale = 4
		p.From.Index = regnum(v.Args[1])
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	case ssa.OpAMD64MOVQstore, ssa.OpAMD64MOVSSstore, ssa.OpAMD64MOVSDstore, ssa.OpAMD64MOVLstore, ssa.OpAMD64MOVWstore, ssa.OpAMD64MOVBstore, ssa.OpAMD64MOVOstore:
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_REG
		p.From.Reg = regnum(v.Args[1])
		p.To.Type = TYPE_MEM
		p.To.Reg = regnum(v.Args[0])
		addAux(&p.To, v)
		progs = append(progs, p)
	case ssa.OpAMD64MOVQstoreidx8, ssa.OpAMD64MOVSDstoreidx8:
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_REG
		p.From.Reg = regnum(v.Args[2])
		p.To.Type = TYPE_MEM
		p.To.Reg = regnum(v.Args[0])
		p.To.Scale = 8
		p.To.Index = regnum(v.Args[1])
		addAux(&p.To, v)
		progs = append(progs, p)
	case ssa.OpAMD64MOVSSstoreidx4:
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_REG
		p.From.Reg = regnum(v.Args[2])
		p.To.Type = TYPE_MEM
		p.To.Reg = regnum(v.Args[0])
		p.To.Scale = 4
		p.To.Index = regnum(v.Args[1])
		addAux(&p.To, v)
		progs = append(progs, p)
	case ssa.OpAMD64MOVQstoreconst, ssa.OpAMD64MOVLstoreconst, ssa.OpAMD64MOVWstoreconst, ssa.OpAMD64MOVBstoreconst:
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_CONST
		sc := ssa.ValAndOff(v.AuxInt)
		i := sc.Val()
		switch v.Op {
		case ssa.OpAMD64MOVBstoreconst:
			i = int64(int8(i))
		case ssa.OpAMD64MOVWstoreconst:
			i = int64(int16(i))
		case ssa.OpAMD64MOVLstoreconst:
			i = int64(int32(i))
		case ssa.OpAMD64MOVQstoreconst:
		}
		p.From.Offset = i
		p.To.Type = TYPE_MEM
		p.To.Reg = regnum(v.Args[0])
		fmt.Println("P.TO.REG:", Rconv(int(p.To.Reg)))
		addAux2(&p.To, v, sc.Off())
		progs = append(progs, p)
	case ssa.OpAMD64MOVLQSX, ssa.OpAMD64MOVWQSX, ssa.OpAMD64MOVBQSX, ssa.OpAMD64MOVLQZX, ssa.OpAMD64MOVWQZX, ssa.OpAMD64MOVBQZX,
		ssa.OpAMD64CVTSL2SS, ssa.OpAMD64CVTSL2SD, ssa.OpAMD64CVTSQ2SS, ssa.OpAMD64CVTSQ2SD,
		ssa.OpAMD64CVTTSS2SL, ssa.OpAMD64CVTTSD2SL, ssa.OpAMD64CVTTSS2SQ, ssa.OpAMD64CVTTSD2SQ,
		ssa.OpAMD64CVTSS2SD, ssa.OpAMD64CVTSD2SS:
		opregreg(int(v.Op.Asm()), regnum(v), regnum(v.Args[0]))
	case ssa.OpAMD64DUFFZERO:
		p = CreateProg(obj.ADUFFZERO)
		p.To.Type = TYPE_ADDR
		//p.To.Sym = Linksym(Pkglookup("duffzero", Runtimepkg))
		p.To.Offset = v.AuxInt
		progs = append(progs, p)
	case ssa.OpAMD64MOVOconst:
		if v.AuxInt != 0 {
			v.Fatalf("MOVOconst can only do constant=0")
		}
		r := regnum(v)
		opregreg(x86.AXORPS, r, r)
	case ssa.OpAMD64DUFFCOPY:
		p = CreateProg(obj.ADUFFCOPY)
		p.To.Type = TYPE_ADDR
		//p.To.Sym = Linksym(Pkglookup("duffcopy", Runtimepkg))
		p.To.Offset = v.AuxInt
		progs = append(progs, p)
	case ssa.OpCopy: // TODO: lower to MOVQ earlier?
		if v.Type.IsMemory() {
			panic("unimplementedf")
			//return
		}
		x := regnum(v.Args[0])
		y := regnum(v)
		if x != y {
			opregreg(regMoveByTypeAMD64(v.Type), y, x)
		}
	case ssa.OpLoadReg:
		if v.Type.IsFlags() {
			v.Fatalf("load flags not implemented: %v", v.LongString())
			panic("unimplementedf")
			//return
		}
		p = CreateProg(movSizeByType(v.Type))
		n, off := autoVar(v.Args[0])
		p.From.Type = TYPE_MEM
		p.From.Node = n
		//p.From.Sym = Linksym(n.Sym)
		p.From.Offset = off
		if n.Class() == PPARAM {
			p.From.Name = NAME_PARAM
			p.From.Offset += n.Xoffset()
		} else {
			p.From.Name = NAME_AUTO
		}
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	case ssa.OpStoreReg:
		if v.Type.IsFlags() {
			v.Fatalf("store flags not implemented: %v", v.LongString())
			panic("unimplementedf")
			//return
		}
		p = CreateProg(movSizeByType(v.Type))
		p.From.Type = TYPE_REG
		p.From.Reg = regnum(v.Args[0])
		n, off := autoVar(v)
		p.To.Type = TYPE_MEM
		p.To.Node = n
		//p.To.Sym = Linksym(n.Sym)
		p.To.Offset = off
		if n.Class() == PPARAM {
			p.To.Name = NAME_PARAM
			p.To.Offset += n.Xoffset()
		} else {
			p.To.Name = NAME_AUTO
		}
		progs = append(progs, p)
	case ssa.OpPhi:
		// just check to make sure regalloc and stackalloc did it right
		if v.Type.IsMemory() {
			panic("unimplementedf")
			//return
		}
		f := v.Block.Func
		loc := f.RegAlloc[v.ID]
		for _, a := range v.Args {
			if aloc := f.RegAlloc[a.ID]; aloc != loc { // TODO: .Equal() instead?
				v.Fatalf("phi arg at different location than phi: %v @ %v, but arg %v @ %v\n%s\n", v, loc, a, aloc, v.Block.Func)
			}
		}
	case ssa.OpConst8, ssa.OpConst16, ssa.OpConst32, ssa.OpConst64, ssa.OpConstString, ssa.OpConstNil, ssa.OpConstBool,
		ssa.OpConst32F, ssa.OpConst64F:
		fmt.Println("v.ID:", v.ID)
		f := v.Block.Func
		fmt.Println("f.RegAlloc:", f.RegAlloc)
		fmt.Println("len(f.RegAlloc):", len(f.RegAlloc))
		if v.Block.Func.RegAlloc[v.ID] != nil {
			v.Fatalf("const value %v shouldn't have a location", v)
		}

	case ssa.OpInitMem:
		// memory arg needs no code
	case ssa.OpArg:
		// input args need no code
	case ssa.OpAMD64LoweredGetClosurePtr:
		// Output is hardwired to DX only,
		// and DX contains the closure pointer on
		// closure entry, and this "instruction"
		// is scheduled to the very beginning
		// of the entry block.
	case ssa.OpAMD64LoweredGetG:
		panic("unimplementedf")
	case ssa.OpAMD64CALLstatic:
		panic("unimplementedf")
	case ssa.OpAMD64CALLclosure:
		panic("unimplementedf")
	case ssa.OpAMD64CALLdefer:
		panic("unimplementedf")
	case ssa.OpAMD64CALLgo:
		panic("unimplementedf")
	case ssa.OpAMD64CALLinter:
		p = CreateProg(obj.ACALL)
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v.Args[0])
		if Maxarg < v.AuxInt {
			Maxarg = v.AuxInt
		}
		progs = append(progs, p)
	case ssa.OpAMD64NEGQ, ssa.OpAMD64NEGL,
		ssa.OpAMD64NOTQ, ssa.OpAMD64NOTL:
		x := regnum(v.Args[0])
		r := regnum(v)
		if x != r {
			p = CreateProg(regMoveAMD64(v.Type.Size()))
			p.From.Type = TYPE_REG
			p.From.Reg = x
			p.To.Type = TYPE_REG
			p.To.Reg = r
		}
		p = CreateProg(int(v.Op.Asm()))
		p.To.Type = TYPE_REG
		p.To.Reg = r
		progs = append(progs, p)
	case ssa.OpAMD64SQRTSD:
		p = CreateProg(int(v.Op.Asm()))
		p.From.Type = TYPE_REG
		p.From.Reg = regnum(v.Args[0])
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	case ssa.OpSP, ssa.OpSB:
		// nothing to do
	case ssa.OpAMD64SETEQ, ssa.OpAMD64SETNE,
		ssa.OpAMD64SETL, ssa.OpAMD64SETLE,
		ssa.OpAMD64SETG, ssa.OpAMD64SETGE,
		ssa.OpAMD64SETGF, ssa.OpAMD64SETGEF,
		ssa.OpAMD64SETB, ssa.OpAMD64SETBE,
		ssa.OpAMD64SETORD, ssa.OpAMD64SETNAN,
		ssa.OpAMD64SETA, ssa.OpAMD64SETAE:
		p = CreateProg(int(v.Op.Asm()))
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		progs = append(progs, p)
	case ssa.OpAMD64SETNEF:
		p = CreateProg(int(v.Op.Asm()))
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		q := CreateProg(x86.ASETPS)
		q.To.Type = TYPE_REG
		q.To.Reg = x86.REG_AX
		// TODO AORQ copied from old code generator, why not AORB?
		opregreg(x86.AORQ, regnum(v), x86.REG_AX)
		progs = append(progs, p)
	case ssa.OpAMD64SETEQF:
		p = CreateProg(int(v.Op.Asm()))
		p.To.Type = TYPE_REG
		p.To.Reg = regnum(v)
		q := CreateProg(x86.ASETPC)
		q.To.Type = TYPE_REG
		q.To.Reg = x86.REG_AX
		// TODO AANDQ copied from old code generator, why not AANDB?
		opregreg(x86.AANDQ, regnum(v), x86.REG_AX)
		progs = append(progs, p)
	case ssa.OpAMD64InvertFlags:
		v.Fatalf("InvertFlags should never make it to codegen %v", v)
	case ssa.OpAMD64REPSTOSQ:
		p := CreateProg(x86.AREP)
		q := CreateProg(x86.ASTOSQ)
		progs = append(progs, p)
		progs = append(progs, q)
	case ssa.OpAMD64REPMOVSQ:
		p := CreateProg(x86.AREP)
		q := CreateProg(x86.AMOVSQ)
		progs = append(progs, p)
		progs = append(progs, q)
	case ssa.OpVarDef:
		panic("unimplementedf")
		//Gvardef(v.Aux.(*Node))
	case ssa.OpVarKill:
		panic("unimplementedf")
		//gvarkill(v.Aux.(*Node))
	case ssa.OpAMD64LoweredNilCheck:
		// Optimization - if the subsequent block has a load or store
		// at the same address, we don't need to issue this instruction.
		for _, w := range v.Block.Succs[0].Block().Values {
			if len(w.Args) == 0 || !w.Args[len(w.Args)-1].Type.IsMemory() {
				// w doesn't use a store - can't be a memory op.
				continue
			}
			if w.Args[len(w.Args)-1] != v.Args[1] {
				v.Fatalf("wrong store after nilcheck v=%s w=%s", v, w)
			}
			switch w.Op {
			case ssa.OpAMD64MOVQload, ssa.OpAMD64MOVLload, ssa.OpAMD64MOVWload, ssa.OpAMD64MOVBload,
				ssa.OpAMD64MOVQstore, ssa.OpAMD64MOVLstore, ssa.OpAMD64MOVWstore, ssa.OpAMD64MOVBstore:
				if w.Args[0] == v.Args[0] && w.Aux == nil && w.AuxInt >= 0 && w.AuxInt < minZeroPage {
					panic("unimplementedf")
					//return
				}
			case ssa.OpAMD64MOVQstoreconst, ssa.OpAMD64MOVLstoreconst, ssa.OpAMD64MOVWstoreconst, ssa.OpAMD64MOVBstoreconst:
				off := ssa.ValAndOff(v.AuxInt).Off()
				if w.Args[0] == v.Args[0] && w.Aux == nil && off >= 0 && off < minZeroPage {
					panic("unimplementedf")
					//return
				}
			}
			if w.Type.IsMemory() {
				// We can't delay the nil check past the next store.
				break
			}
		}
		// Issue a load which will fault if the input is nil.
		// TODO: We currently use the 2-byte instruction TESTB AX, (reg).
		// Should we use the 3-byte TESTB $0, (reg) instead?  It is larger
		// but it doesn't have false dependency on AX.
		// Or maybe allocate an output register and use MOVL (reg),reg2 ?
		// That trades clobbering flags for clobbering a register.
		p = CreateProg(x86.ATESTB)
		p.From.Type = TYPE_REG
		p.From.Reg = x86.REG_AX
		p.To.Type = TYPE_MEM
		p.To.Reg = regnum(v.Args[0])
		addAux(&p.To, v)
		progs = append(progs, p)
	default:
		fmt.Println("unimplemented OP:", v.Op.String())
		v.Fatalf("genValue not implemented: %s", v.LongString())
		panic("unimplementedf")

	}
	return progs
}

// movSizeByType returns the MOV instruction of the given type.
func movSizeByType(t ssa.Type) (asm int) {
	// For x86, there's no difference between reg move opcodes
	// and memory move opcodes.
	asm = regMoveByTypeAMD64(t)
	return
}

// movZero generates a register indirect move with a 0 immediate and keeps track of bytes left and next offset
/*func movZero(as int, width int64, nbytes int64, offset int64, regnum int16) (nleft int64, noff int64) {
	p := Prog(as)
	// TODO: use zero register on archs that support it.
	p.From.Type = TYPE_CONST
	p.From.Offset = 0
	p.To.Type = TYPE_MEM
	p.To.Reg = regnum
	p.To.Offset = offset
	offset += width
	nleft = nbytes - width
	return nleft, offset
}*/

var blockJump = [...]struct {
	asm, invasm int
}{
	ssa.BlockAMD64EQ:  {x86.AJEQ, x86.AJNE},
	ssa.BlockAMD64NE:  {x86.AJNE, x86.AJEQ},
	ssa.BlockAMD64LT:  {x86.AJLT, x86.AJGE},
	ssa.BlockAMD64GE:  {x86.AJGE, x86.AJLT},
	ssa.BlockAMD64LE:  {x86.AJLE, x86.AJGT},
	ssa.BlockAMD64GT:  {x86.AJGT, x86.AJLE},
	ssa.BlockAMD64ULT: {x86.AJCS, x86.AJCC},
	ssa.BlockAMD64UGE: {x86.AJCC, x86.AJCS},
	ssa.BlockAMD64UGT: {x86.AJHI, x86.AJLS},
	ssa.BlockAMD64ULE: {x86.AJLS, x86.AJHI},
	ssa.BlockAMD64ORD: {x86.AJPC, x86.AJPS},
	ssa.BlockAMD64NAN: {x86.AJPS, x86.AJPC},
}

/*type floatingEQNEJump struct {
	jump, index int
}

var eqfJumps = [2][2]floatingEQNEJump{
	{{x86.AJNE, 1}, {x86.AJPS, 1}}, // next == b.Succs[0]
	{{x86.AJNE, 1}, {x86.AJPC, 0}}, // next == b.Succs[1]
}
var nefJumps = [2][2]floatingEQNEJump{
	{{x86.AJNE, 0}, {x86.AJPC, 1}}, // next == b.Succs[0]
	{{x86.AJNE, 0}, {x86.AJPS, 0}}, // next == b.Succs[1]
}
*/

/*func oneFPJump(b *ssa.Block, jumps *floatingEQNEJump, likely ssa.BranchPrediction, branches []branch) []branch {
	p := Prog(jumps.jump)
	p.To.Type = TYPE_BRANCH
	to := jumps.index
	branches = append(branches, branch{p, b.Succs[to]})
	if to == 1 {
		likely = -likely
	}
	// liblink reorders the instruction stream as it sees fit.
	// Pass along what we know so liblink can make use of it.
	// TODO: Once we've fully switched to SSA,
	// make liblink leave our output alone.
	switch likely {
	case ssa.BranchUnlikely:
		p.From.Type = TYPE_CONST
		p.From.Offset = 0
	case ssa.BranchLikely:
		p.From.Type = TYPE_CONST
		p.From.Offset = 1
	}
	return branches
}*/

/*func genFPJump(s *genState, b, next *ssa.Block, jumps *[2][2]floatingEQNEJump) {
	likely := b.Likely
	switch next {
	case b.Succs[0]:
		s.branches = oneFPJump(b, &jumps[0][0], likely, s.branches)
		s.branches = oneFPJump(b, &jumps[0][1], likely, s.branches)
	case b.Succs[1]:
		s.branches = oneFPJump(b, &jumps[1][0], likely, s.branches)
		s.branches = oneFPJump(b, &jumps[1][1], likely, s.branches)
	default:
		s.branches = oneFPJump(b, &jumps[1][0], likely, s.branches)
		s.branches = oneFPJump(b, &jumps[1][1], likely, s.branches)
		q := Prog(obj.AJMP)
		q.To.Type = TYPE_BRANCH
		s.branches = append(s.branches, branch{q, b.Succs[1]})
	}
}*/

func (s *genState) genBlock(b, next *ssa.Block) []*Prog {
	var progs []*Prog

	switch b.Kind {
	case ssa.BlockPlain:
		if b.Succs[0].Block() != next {
			p := CreateProg(obj.AJMP)
			p.To.Type = TYPE_BRANCH
			s.branches = append(s.branches, branch{p, b.Succs[0].Block()})
			progs = append(progs, p)
		}
	case ssa.BlockExit:
		progs = append(progs, CreateProg(obj.AUNDEF)) // tell plive.go that we never reach here
	case ssa.BlockRet:
		if hasdefer {
			panic("defer unsupported")
			//s.deferReturn()
		}
		progs = append(progs, CreateProg(obj.ARET))
	case ssa.BlockRetJmp:
		p := CreateProg(obj.AJMP)
		p.To.Type = TYPE_MEM
		p.To.Name = NAME_EXTERN
		//p.To.Sym = Linksym(b.Aux.(*Sym))
		progs = append(progs, p)

	case ssa.BlockAMD64EQF:
		panic("unimplementedf")
		//genFPJump(s, b, next, &eqfJumps)

	case ssa.BlockAMD64NEF:
		panic("unimplementedf")
		//genFPJump(s, b, next, &nefJumps)

	case ssa.BlockAMD64EQ, ssa.BlockAMD64NE,
		ssa.BlockAMD64LT, ssa.BlockAMD64GE,
		ssa.BlockAMD64LE, ssa.BlockAMD64GT,
		ssa.BlockAMD64ULT, ssa.BlockAMD64UGT,
		ssa.BlockAMD64ULE, ssa.BlockAMD64UGE:
		jmp := blockJump[b.Kind]
		likely := b.Likely
		var p *Prog
		switch next {
		case b.Succs[0].Block():
			p = CreateProg(jmp.invasm)
			likely *= -1
			p.To.Type = TYPE_BRANCH
			s.branches = append(s.branches, branch{p, b.Succs[1].Block()})
		case b.Succs[1].Block():
			p = CreateProg(jmp.asm)
			p.To.Type = TYPE_BRANCH
			s.branches = append(s.branches, branch{p, b.Succs[0].Block()})
		default:
			p = CreateProg(jmp.asm)
			p.To.Type = TYPE_BRANCH
			s.branches = append(s.branches, branch{p, b.Succs[0].Block()})
			q := CreateProg(obj.AJMP)
			q.To.Type = TYPE_BRANCH
			s.branches = append(s.branches, branch{q, b.Succs[1].Block()})
		}

		// liblink reorders the instruction stream as it sees fit.
		// Pass along what we know so liblink can make use of it.
		// TODO: Once we've fully switched to SSA,
		// make liblink leave our output alone.
		switch likely {
		case ssa.BranchUnlikely:
			p.From.Type = TYPE_CONST
			p.From.Offset = 0
		case ssa.BranchLikely:
			p.From.Type = TYPE_CONST
			p.From.Offset = 1
		}
		progs = append(progs, p)
	default:
		panic("unimplemented")
		//b.Unimplementedf("branch not implemented: %s. Control: %s", b.LongString(), b.Control.LongString())
	}
	return progs
}

func (s *genState) deferReturn() {
	// Deferred calls will appear to be returning to
	// the CALL deferreturn(SB) that we are about to emit.
	// However, the stack trace code will show the line
	// of the instruction byte before the return PC.
	// To avoid that being an unrelated instruction,
	// insert an actual hardware NOP that will have the right line number.
	// This is different from obj.ANOP, which is a virtual no-op
	// that doesn't make it into the instruction stream.
	/*s.deferTarget = Pc
	Thearch.Ginsnop()
	p := Prog(obj.ACALL)
	p.To.Type = TYPE_MEM
	p.To.Name = obj.NAME_EXTERN
	p.To.Sym = Linksym(Deferreturn.Sym)*/
}

// addAux adds the offset in the aux fields (AuxInt and Aux) of v to a.
func addAux(a *Addr, v *ssa.Value) {
	addAux2(a, v, v.AuxInt)
}

func addAux2(a *Addr, v *ssa.Value, offset int64) {
	if a.Type != TYPE_MEM {
		v.Fatalf("bad addAux addr %v", a)
	}
	// add integer offset
	a.Offset += offset

	// If no additional symbol offset, we're done.
	if v.Aux == nil {
		return
	}
	// Add symbol's offset from its base register.
	switch sym := v.Aux.(type) {
	case *ssa.ExternSymbol:
		a.Name = NAME_EXTERN
		//a.Sym = Linksym(sym.Sym.(*Sym))
	case *ssa.ArgSymbol:
		n := sym.Node.(ssaVar)
		a.Name = NAME_PARAM
		a.Node = n
		a.Sym = &LSym{} //Linksym(n.Orig.Sym)
		a.Sym.Name = n.Name()
		a.Offset += n.Xoffset() // TODO: why do I have to add this here?  I don't for auto variables.
	case *ssa.AutoSymbol:
		n := sym.Node.(ssaVar)
		a.Name = NAME_AUTO
		a.Node = n
		//a.Sym = Linksym(n.Sym)
	default:
		v.Fatalf("aux in %s not implemented %#v", v, v.Aux)
	}
}

// ssaRegToReg maps ssa register numbers to obj register numbers.
var ssaRegToReg = [...]int16{
	x86.REG_AX,
	x86.REG_CX,
	x86.REG_DX,
	x86.REG_BX,
	x86.REG_SP,
	x86.REG_BP,
	x86.REG_SI,
	x86.REG_DI,
	x86.REG_R8,
	x86.REG_R9,
	x86.REG_R10,
	x86.REG_R11,
	x86.REG_R12,
	x86.REG_R13,
	x86.REG_R14,
	x86.REG_R15,
	x86.REG_X0,
	x86.REG_X1,
	x86.REG_X2,
	x86.REG_X3,
	x86.REG_X4,
	x86.REG_X5,
	x86.REG_X6,
	x86.REG_X7,
	x86.REG_X8,
	x86.REG_X9,
	x86.REG_X10,
	x86.REG_X11,
	x86.REG_X12,
	x86.REG_X13,
	x86.REG_X14,
	x86.REG_X15,
	0, // SB isn't a real register.  We fill an Addr.Reg field with 0 in this case.
	// TODO: arch-dependent
}

// regMoveAMD64 returns the register->register move opcode for the given width.
// TODO: generalize for all architectures?
func regMoveAMD64(width int64) int {
	switch width {
	case 1:
		return x86.AMOVB
	case 2:
		return x86.AMOVW
	case 4:
		return x86.AMOVL
	case 8:
		return x86.AMOVQ
	default:
		panic("bad int register width")
	}
}

func regMoveByTypeAMD64(t ssa.Type) int {
	width := t.Size()
	if t.IsFloat() {
		switch width {
		case 4:
			return x86.AMOVSS
		case 8:
			return x86.AMOVSD
		default:
			panic("bad float register width")
		}
	} else {
		switch width {
		case 1:
			return x86.AMOVB
		case 2:
			return x86.AMOVW
		case 4:
			return x86.AMOVL
		case 8:
			return x86.AMOVQ
		default:
			panic("bad int register width")
		}
	}

	// panic("bad register type")
}

// regnum returns the register (in cmd/internal/obj numbering) to
// which v has been allocated.  Panics if v is not assigned to a
// register.
// TODO: Make this panic again once it stops happening routinely.
/*func regnum(v *ssa.Value) int16 {
	reg := v.Block.Func.RegAlloc[v.ID]
	if reg == nil {
		v.Fatalf("nil regnum for value: %s\n%s\n", v.LongString(), v.Block.Func)
		return 0
	}
	return ssaRegToReg[reg.(*ssa.Register).Num]
}*/

// autoVar returns a *Node and int64 representing the auto variable and offset within it
// where v should be spilled.
/*func autoVar(v *ssa.Value) (*Node, int64) {
	loc := v.Block.Func.RegAlloc[v.ID].(ssa.LocalSlot)
	return loc.N.(*Node), loc.Off
}*/
