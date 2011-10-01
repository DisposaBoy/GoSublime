import sublime, sublime_plugin
import json, os
import gscommon as gs
from os.path import basename

class GoSublime(sublime_plugin.EventListener):
    gocode_set = False
    def on_query_completions(self, view, prefix, locations):
        pos = locations[0]
        scopes = view.scope_name(pos).split()
        if 'source.go' not in scopes:
            return []
        
        # if we complete inside e.g. a map's key we're going to cause subtle bugs so bail
        if 'string.quoted.double.go' in scopes or 'string.quoted.single.go' in scopes or 'string.quoted.raw.go' in scopes:
            # afaik we must return something in order to disable st2's word completion
            return [(' ', '$0')]

        if not self.gocode_set:
            self.gocode_set = True
            # autostart the daemon
            gs.runcmd([gs.setting('gocode_cmd', 'gocode')])

        # gocode is case-sesitive so push the location back to the 'dot' so it gives
        # gives us everything then st2 can pick the matches for us
        offset = pos - len(prefix)
        src = view.substr(sublime.Region(0, view.size()))
        fn = view.file_name()
        cl = self.complete(fn, offset, src)

        if scopes[-1] == 'source.go':
            cl.extend(gs.GLOBAL_SNIPPETS)
        elif scopes[-1] == 'meta.block.go' and ('meta.function.plain.go' in scopes or 'meta.function.receiver.go' in scopes):
            cl.extend(gs.LOCAL_SNIPPETS)
        
        return cl
    
    def complete(self, fn, offset, src):
        comps = []
        cmd = gs.setting('gocode_cmd', 'gocode')
        can_pass_char_offset = gs.setting('gocode_accepts_character_offsets', False)
        if can_pass_char_offset is True:
            offset = 'c%s' % offset
        else:
            offset = gs.char_to_byte_offset(src, offset)
        args = [cmd, "-f=json", "autocomplete", fn, str(offset)]
        js, err = gs.runcmd(args, src)
        if err:
            sublime.error_message(err)
        else:
            try:    
                js = json.loads(js)
                if js and js[1]:
                    for ent in js[1]:
                        if ent['name'] == 'main':
                            continue
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
            except ValueError as e:
                sublime.error_message("Error while decoding gocode output: %s" % e)
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
