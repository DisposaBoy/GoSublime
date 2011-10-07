import sublime, sublime_plugin
import gscommon as gs
import re, threading
import os
import string

LINE_PAT = re.compile(r'^(.+):(\d+):(\d+):\s+(.+)\s*$', re.MULTILINE)

class GsLint(sublime_plugin.EventListener):
    rc = 0
    errors = {}

    def on_selection_modified(self, view):
        sel = view.sel()[0].begin()
        if view.score_selector(sel, 'source.go') > 0:
            line = view.rowcol(sel)[0]
            msg = self.errors.get(view.id(), {}).get(line, '')
            view.set_status('GsLint', ('GsLint: ' + msg) if msg else '')
    
    def on_modified(self, view):
        pos = view.sel()[0].begin()
        scopes = view.scope_name(pos).split()
        if 'source.go' in scopes:
            self.rc += 1

            should_run = (
                         'string.quoted.double.go' not in scopes and
                         'string.quoted.single.go' not in scopes and
                         'string.quoted.raw.go' not in scopes and
                         'comment.line.double-slash.go' not in scopes and
                         'comment.block.go' not in scopes
            )

            def cb():
                self.lint(view)
            
            if should_run:
                sublime.set_timeout(cb, int(gs.setting('gslint_timeout', 500)))
            else:
                # we want to cleanup if e.g settings changed or we caused an error entering an excluded scope
                sublime.set_timeout(cb, 1000)
    
    def on_load(self, view):
        self.on_modified(view)
    
    #taken from http://stackoverflow.com/questions/241327/python-snippet-to-remove-c-and-c-comments/241506#241506
    def comment_remover(self, text):
        def replacer(match):
            s = match.group(0)
            if s.startswith('/'):
                return ""
            else:
                return s
        pattern = re.compile(
            r'//.*?$|/\*.*?\*/|\'(?:\\.|[^\\\'])*\'|"(?:\\.|[^\\"])*"',
            re.DOTALL | re.MULTILINE
        )
        return re.sub(pattern, replacer, text)

    #extracts the go package name from a go source file
    def extract_go_package_name(self, filename):
        fd = open(filename, "r")
        content = string.join(fd.readlines())
        fd.close()
        strippedcontent = self.comment_remover(content)
        for line in strippedcontent.split("\n"):
            sline = line.strip()
            if sline.find("package") == 0:
                return sline.split(" ")[1]
        return ""   

    def generate_filelist_for_lint(self, view):
        #input filename
        filename = view.file_name()
        #list of files to run lint on
        lintfiles = [filename]
        basepath = os.path.dirname(filename)
        packagename = self.extract_go_package_name(filename)

        for name in os.listdir(basepath):
            newfile = os.path.join(basepath,name)
            if newfile == filename:
                continue
            if os.path.isfile(newfile):
                _, ext = os.path.splitext(name)
                if ext.lower() == ".go":
                    if self.extract_go_package_name(newfile) == packagename:
                        lintfiles += [newfile]
        return lintfiles

    def lint(self, view):
        self.rc -= 1

        if self.rc == 0:
            cmd = gs.setting('gslint_cmd', 'gotype')
            if cmd:
                filelist = self.generate_filelist_for_lint(view)
                filelist = [cmd] + filelist
                _, err = gs.runcmd(filelist, "")
            else:
                err = ''
            lines = LINE_PAT.findall(err)
            regions = []
            view_id = view.id()        
            self.errors[view_id] = {}
            if lines:
                for m in lines:
                    if m[0] == view.file_name(): #check if this error message is for the current file
                        line, start, err = int(m[1])-1, int(m[2])-1, m[3]
                        self.errors[view_id][line] = err
                        lr = view.line(view.text_point(line, start))
                        regions.append(sublime.Region(lr.begin() + start, lr.end()))
            if regions:
                flags = sublime.DRAW_EMPTY_AS_OVERWRITE | sublime.DRAW_OUTLINED
                flags = sublime.DRAW_EMPTY_AS_OVERWRITE
                flags = sublime.DRAW_OUTLINED
                view.add_regions('GsLint-errors', regions, 'invalid.illegal', 'cross', flags)
            else:
                view.erase_regions('GsLint-errors')
        self.on_selection_modified(view)
