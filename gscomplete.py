import sublime, sublime_plugin
import json, os
import gscommon as gs
from os.path import basename

class GoSublime(sublime_plugin.EventListener):
    def on_query_completions(self, view, prefix, locations):
        pos = locations[0]
        scopes = view.scope_name(pos).split()
        if 'source.go' not in scopes:
            return []

        # gocode is case-sesitive so push the location back to the 'dot' so it gives
        # gives us everything then st2 can pick the matches for us
        offset = str(pos - len(prefix))
        src = view.substr(sublime.Region(0, view.size()))
        fn = basename(view.file_name())
        cl = self.complete(fn, offset, src)

        if len(scopes) == 1:
            cl.extend(gs.GLOBAL_SNIPPETS)
        else:
            cl.extend(gs.LOCAL_SNIPPETS)
        
        return cl
    
    def complete(self, fn, offset, src):
        comps = []
        cmd = gs.setting('gocode_cmd', 'gocode')
        args = [cmd, "-f=json", "autocomplete", fn, offset]
        js, err = gs.runcmd(args, src)
        try:
            if err:
                sublime.error_message(err)
            else:
                js = json.loads(js)
                if js and js[1]:
                    for ent in js[1]:
                        etype = ent['type']
                        eclass = ent['class']
                        ename = ent['name']
                        tname = self.typeclass_prefix(eclass, etype) + ename
                        if ent['class'] == 'func':
                            comps.append(self.parse_decl_hack(etype, ename, tname))
                        elif ent['class'] != 'PANIC':
                            comps.append((tname, ename))
        except KeyError as e:
            sublime.error_message('Error while running gocode, possibly malformed data returned: %s' % e)
        return comps
    
    def parse_decl_hack(self, s, name, tname):
        # this will go if/when there is sublime output support in gocode 
        p_count = 0
        lp_index = -1
        rp_index = -1

        for i, c in enumerate(s):
            if c == '(':
                if p_count == 0:
                    lp_index = i + 1
                p_count += 1
            elif c == ')':
                if p_count == 1:
                    rp_index = i
                    break
                p_count -= 1

        if lp_index >= 0 and  rp_index >= 1:
            decl = []
            args = s[lp_index:rp_index].split(',')
            for i, a in enumerate(args):
                a = a.strip().replace('{', '\\{').replace('}', '\\}')
                decl.append('${%d:%s}' % (i+1, a))
            if decl:
                return (tname, '%s(%s)' % (name, ', '.join(decl)))
        return (tname, name)
    
    def typeclass_prefix(self, typeclass, typename):
        return gs.NAME_PREFIXES.get(typename, gs.CLASS_PREFIXES.get(typeclass, ''))
