import sublime, sublime_plugin
import subprocess, json, os

GLOBAL_SNIPPETS = [
    (u'\u0282  func: Function ...', 'func ${1:name}($2)$3 {\n\t$0\n}'),
    (u'\u0282  func: Function (receiver) ...', 'func (${1:receiver} ${2:type}) ${3:name}($4)$5 {\n\t$0\n}'),
    (u'\u0282  var: Variable (...)', 'var (\n\t$1\n)'),
    (u'\u0282  const: Constant (...)', 'const (\n\t$1\n)'),
    (u'\u0282  import: Import (...)', 'import (\n\t$2"$1"\n)'),
    (u'\u0282  package: Package ...', 'package ${1:NAME}')
]

LOCAL_SNIPPETS = [
    (u'\u0282  func: Function() ...', 'func($1) {\n\t$0\n}($2)'),
    (u'\u0282  var: Variable (...)', 'var ${1:name} ${2:type}'),
]

CLASS_PREFIXES = {
    'const': u'\u0196   ',
    'func': u'\u0192   ',
    'type': u'\u0288   ',
    'var':  u'\u03BD  ',
    'package': u'\u03C1  ',
}

NAME_PREFIXES = {
    'interface': u'\u00A1  ',
}

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
        fn = os.path.basename(view.file_name())
        cl = self.complete(fn, offset, src)

        if len(scopes) == 1:
            cl.extend(GLOBAL_SNIPPETS)
        else:
            cl.extend(LOCAL_SNIPPETS)
        
        return cl
    
    def complete(self, fn, offset, src):
        args = ["gocode", "-f=json", "autocomplete", fn, offset]
        try:
            p = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, stdin=subprocess.PIPE)
            streams = p.communicate(input=src)
            js = json.loads(streams[0])
            if js:
                comps = []
                for e in js[1]:
                    name = e['name']
                    tname = self.typeclass_prefix(e['class'], e['type']) + name
                    typeclass = e['class']
                    if typeclass == 'func':
                        comps.append(self.parse_decl_hack(e['type'], name, tname))
                    elif typeclass != 'PANIC':
                        comps.append((tname, name))
                return comps
        except (OSError, ValueError) as e:
            sublime.error_message('Error while running gocode: %s' % e)
        except KeyError as e:
            sublime.error_message('Error while running gocode, possibly malformed data returned: %s' % e)

        return []
    
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
        return NAME_PREFIXES.get(typename, CLASS_PREFIXES.get(typeclass, ''))
