package golang

import (
	"fmt"
	"go/ast"
	"go/scanner"
	"go/token"
	"go/types"
	"margo.sh/golang/goutil"
	"margo.sh/htm"
	kim "margo.sh/kimporter"
	"margo.sh/mg"
	"margo.sh/mgpf"
	"margo.sh/mgutil"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	typChkR = &typChk{
		cfg: TypeCheck{
			NoIssues:  true,
			NoInfo:    true,
			NoGotoDef: true,
		},
	}
)

func init() {
	mg.DefaultReducers.Before(typChkR)
}

type tcInfo struct {
	Id  *ast.Ident
	Obj types.Object
	Pkg *kim.Package
}

type TypeCheck struct {
	mg.ReducerType
	NoIssues  bool
	NoInfo    bool
	NoGotoDef bool
}

func (tc *TypeCheck) RInit(mx *mg.Ctx) {
	typChkR.configure(*tc)
}

func (tc *TypeCheck) Reduce(mx *mg.Ctx) *mg.State {
	return mx.State
}

type typChk struct {
	mg.ReducerType

	isuQ *mgutil.ChanQ

	infQ *mgutil.ChanQ

	mu    sync.Mutex
	cfg   TypeCheck
	infEl htm.Element
}

func (tc *typChk) configure(c TypeCheck) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.cfg = c
}

func (tc *typChk) config() TypeCheck {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	return tc.cfg
}

func (tc *typChk) RCond(mx *mg.Ctx) bool {
	return mx.LangIs(mg.Go)
}

func (tc *typChk) RMount(mx *mg.Ctx) {
	tc.isuQ = mgutil.NewChanQLoop(1, func(mx interface{}) {
		if !tc.config().NoIssues {
			tc.isuProc(mx.(*mg.Ctx))
		}
	})
	tc.infQ = mgutil.NewChanQLoop(1, func(mx interface{}) {
		if !tc.config().NoInfo {
			tc.infProc(mx.(*mg.Ctx))
		}
	})
}

func (tc *typChk) RUnmount(mx *mg.Ctx) {
	tc.isuQ.Close()
	tc.infQ.Close()
}

func (tc *typChk) Reduce(mx *mg.Ctx) *mg.State {
	st := mx.State
	switch act := mx.Action.(type) {
	case mg.ViewActivated:
		tc.isuQ.Put(mx)
		tc.infQ.Put(mx)
	case mg.ViewModified, mg.ViewSaved:
		tc.isuQ.Put(mx)
	case mg.ViewPosChanged:
		tc.infQ.Put(mx)
	case mg.QueryUserCmds:
		st = st.AddUserCmds(
			mg.UserCmd{
				Title: "Goto Definition",
				Name:  "typecheck.definition",
				Desc:  "Go to the declaration of selected identifier",
			},
		)
	case mg.RunCmd:
		st = tc.handleRunCmd(mx, st, act)
	}

	tc.mu.Lock()
	if tc.infEl != nil {
		st = st.AddHUD(htm.Text("TypeInfo"), tc.infEl)
	}
	tc.mu.Unlock()

	return st
}

func (tc *typChk) handleRunCmd(mx *mg.Ctx, st *mg.State, rc mg.RunCmd) *mg.State {
	ok := !tc.config().NoGotoDef && (rc.Name == "goto.definition" ||
		rc.Name == "typecheck.definition" ||
		(rc.Name == mg.RcActuate && rc.StringFlag("button", "left") == "left"))
	if !ok {
		return st
	}
	return st.AddBuiltinCmds(mg.BuiltinCmd{
		Name: rc.Name,
		Run: func(cx *mg.CmdCtx) *mg.State {
			go tc.gotoDef(cx)
			return cx.State
		}},
	)
}

func (tc *typChk) gotoDef(cx *mg.CmdCtx) {
	defer cx.Output.Close()

	// TODO: maybe make infProc handle this
	ti, ok := tc.info(cx.Ctx)
	if !ok {
		fmt.Fprintln(cx.Output, "TypeCheck: Object type not found.")
		return
	}
	tp, act, ok := tc.defAct(ti)
	if !ok {
		fmt.Fprintln(cx.Output, "TypeCheck: Declaration not found.")
		return
	}
	fmt.Fprintf(cx.Output, "TypeCheck: Identifier: %s, Definition: %s", ti.Id, tp)
	cx.Store.Dispatch(act)
}

func (tc *typChk) isuProc(mx *mg.Ctx) {
	defer mx.Begin(mg.Task{Title: "Go/TypeCheck"}).Done()
	pf := mgpf.NewProfile("Go/TypeCheck")
	defer func() {
		if pf.Dur().Duration < 100*time.Millisecond {
			return
		}
		mx.Profile.Fprint(os.Stderr, &mgpf.PrintOpts{
			MinDuration: 10 * time.Millisecond,
		})
	}()
	mx = mx.Copy(func(mx *mg.Ctx) { mx.Profile = pf })
	v := mx.View

	src, _ := v.ReadAll()
	issues := []mg.Issue{}
	if v.Path == "" {
		pf := goutil.ParseFile(mx, v.Name, src)
		issues = append(issues, tc.errToIssues(mx, v, pf.Error)...)
		if pf.Error == nil {
			tcfg := types.Config{
				IgnoreFuncBodies: true,
				FakeImportC:      true,
				Error: func(err error) {
					issues = append(issues, tc.errToIssues(mx, v, err)...)
				},
				Importer: kim.New(mx, nil),
			}
			tcfg.Check("_", pf.Fset, []*ast.File{pf.AstFile}, nil)
		}
	} else {
		kp := kim.New(mx, &kim.Config{
			CheckFuncs:   true,
			CheckImports: true,
			Tests:        strings.HasSuffix(v.Filename(), "_test.go"),
			SrcMap:       map[string][]byte{v.Filename(): src},
		})
		_, err := kp.ImportFrom(".", v.Dir(), 0)
		issues = append(issues, tc.errToIssues(mx, v, err)...)
	}
	for i, isu := range issues {
		if isu.Path == "" {
			isu.Path = v.Path
			isu.Name = v.Name
		}
		isu.Label = "Go/typeCheck"
		isu.Tag = mg.Error
		issues[i] = isu
	}

	type K struct{}
	mx.Store.Dispatch(mg.StoreIssues{
		IssueKey: mg.IssueKey{Key: K{}},
		Issues:   issues,
	})
}

func (tc *typChk) errToIssues(mx *mg.Ctx, v *mg.View, err error) mg.IssueSet {
	var issues mg.IssueSet
	switch e := err.(type) {
	case nil:
	case scanner.ErrorList:
		for _, err := range e {
			issues = append(issues, tc.errToIssues(mx, v, err)...)
		}
	case mg.Issue:
		if e.Name == "" && e.Path == "" {
			// guard against failure to set .Path
			e.Name = v.Name
		}
		issues = append(issues, e)
	case scanner.Error:
		issues = append(issues, tc.posIssue(mx, v, e.Msg, e.Pos))
	case *scanner.Error:
		issues = append(issues, tc.posIssue(mx, v, e.Msg, e.Pos))
	case types.Error:
		issues = append(issues, tc.posIssue(mx, v, e.Msg, e.Fset.Position(e.Pos)))
	case *types.Error:
		issues = append(issues, tc.posIssue(mx, v, e.Msg, e.Fset.Position(e.Pos)))
	default:
		issues = append(issues, mg.Issue{
			Name:    v.Name,
			Message: err.Error(),
		})
	}
	return issues
}

func (tc *typChk) infProc(mx *mg.Ctx) {
	pf := mgpf.NewProfile("Go/TypeInfo")
	mx = mx.Copy(func(mx *mg.Ctx) { mx.Profile = pf })
	defer mx.Begin(mg.Task{Title: "Go/TypeInfo"}).Done()
	defer func() {
		if pf.Dur().Duration < 50*time.Millisecond {
			return
		}
		mx.Profile.Fprint(os.Stderr, &mgpf.PrintOpts{
			MinDuration: 10 * time.Millisecond,
		})
	}()
	defer mx.Store.Dispatch(mg.Render)

	ti, ok := tc.info(mx)
	tc.mu.Lock()
	if ok {
		tc.infEl = tc.infHUD(mx, ti)
	} else {
		tc.infEl = nil
	}
	tc.mu.Unlock()
}

func (tc *typChk) typesInfo() kim.TypesInfo {
	// NOTE: to reduce memory use, we don't load everything by default
	// so these flags need updating depending on what types.Info fields are used
	return kim.TypesInfoDefs | kim.TypesInfoUses
}

func (tc *typChk) info(mx *mg.Ctx) (*tcInfo, bool) {
	// TODO: caching?
	// kimporter's caching should be fast enough to allow us to do this on every ViewPosChanged
	v := mx.View
	src, _ := v.ReadAll()

	pf := goutil.ParseFile(mx, v.Name, src)
	if pos := pf.TokenFile.Pos(v.Pos); !pos.IsValid() || goutil.IdentAt(pf.AstFile, pos) == nil {
		return nil, false
	}

	ti := &tcInfo{}
	// TODO: merge these
	if v.Path == "" {
		tcfg := types.Config{
			IgnoreFuncBodies: true,
			FakeImportC:      true,
			Error:            func(err error) {},
			Importer: kim.New(mx, &kim.Config{
				TypesInfo: tc.typesInfo(),
			}),
		}
		tinf := tc.typesInfo().New()
		p, _ := tcfg.Check("_", pf.Fset, []*ast.File{pf.AstFile}, tinf)
		ti.Pkg = kim.NewPackage(
			p,
			pf.Fset,
			map[string]*ast.File{v.Filename(): pf.AstFile},
			tinf,
			nil,
		)
	} else {
		ti.Pkg, _ = kim.New(mx, &kim.Config{
			CheckFuncs:   true,
			CheckImports: true,
			Tests:        strings.HasSuffix(v.Filename(), "_test.go"),
			SrcMap:       map[string][]byte{v.Filename(): src},
			TypesInfo:    tc.typesInfo(),
		}).ImportPackage(".", v.Dir())
	}
	if ti.Pkg == nil || ti.Pkg.Fset == nil || ti.Pkg.Info == nil {
		return nil, false
	}
	af := ti.Pkg.Files[mx.View.Basename()]
	if af == nil {
		return nil, false
	}
	tf := ti.Pkg.Fset.File(af.Pos())
	if tf == nil {
		return nil, false
	}
	pos := tf.Pos(mx.View.Pos)
	if !pos.IsValid() {
		return nil, false
	}
	ti.Id = goutil.IdentAt(af, pos)
	if ti.Id == nil {
		return nil, false
	}
	ti.Obj = ti.Pkg.Info.ObjectOf(ti.Id)
	if ti.Obj == nil {
		return nil, false
	}
	ti.Obj, ti.Pkg = tc.objPkg(mx, ti.Obj, ti.Pkg)
	if ti.Pkg == nil {
		return nil, false
	}
	if ti.Pkg.Fset == nil {
		return nil, false
	}
	return ti, true
}

func (tc *typChk) infHUD(mx *mg.Ctx, ti *tcInfo) htm.Element {
	els := []htm.IElement{}
	addEl := func(pfx string, val htm.IElement) {
		if len(els) != 0 {
			els = append(els, htm.Text(", "))
		}
		els = append(els, htm.Span(nil,
			htm.Strong(nil, htm.Text(pfx)),
			val,
		))
	}

	addEl("Sel: ", htm.Text(ti.Id.String()))
	if t := ti.Obj.Type(); t != nil {
		addEl("Type: ", htm.Text(t.String()))
	}
	if tp, act, ok := tc.defAct(ti); ok {
		s := mgutil.ShortFn(tp.String(), mx.Env)
		addEl("Def: ", htm.A(&htm.AAttrs{Action: act}, htm.Text(s)))
	}
	return htm.Span(nil, els...)
}

func (tc *typChk) defAct(ti *tcInfo) (token.Position, mg.Activate, bool) {
	tp := ti.Pkg.Fset.Position(ti.Obj.Pos())
	if !tp.IsValid() {
		return token.Position{}, mg.Activate{}, false
	}
	return tp, mg.Activate{
		Path: tp.Filename,
		Row:  tp.Line - 1,
		Col:  tp.Column - 1,
	}, true
}

func (tc *typChk) objPkg(mx *mg.Ctx, obj types.Object, pkg *kim.Package) (types.Object, *kim.Package) {
	if obj == nil {
		return obj, nil
	}
	p := obj.Pkg()
	if p == nil {
		pb := kim.PkgBuiltin()
		if obj := pb.Scope().Lookup(obj.Name()); obj != nil {
			return obj, pb
		}
		return obj, nil
	}
	if p == pkg.Package {
		return obj, pkg
	}
	if q := pkg.Imports[p.Path()]; q != nil {
		return obj, q
	}
	q, _ := kim.New(mx, nil).
		ImportPackage(p.Path(), mx.View.Dir())
	return obj, q
}

func (tc *typChk) posIssue(mx *mg.Ctx, v *mg.View, msg string, p token.Position) mg.Issue {
	is := mg.Issue{
		Path:    p.Filename,
		Row:     p.Line - 1,
		Col:     p.Column - 1,
		Message: msg,
	}
	if is.Path == "" {
		is.Name = v.Name
	}
	return is
}
