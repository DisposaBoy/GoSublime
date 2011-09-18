import sublime, sublime_plugin
import gscommon as gs
import thatcher

class SublimeEditor(thatcher.Editor):
    def __init__(self, view, edit):
        self.view = view
        self.edit = edit

    def update_regions(self):
        self.regions = self.view.split_by_newlines(sublime.Region(0, self.view.size()))

    def match_line(self, line_index, content):
        self.update_regions()
        if line_index < len(self.regions):
            return self.view.substr(self.regions[line_index]) == content
        return False

    def insert_line(self, line_index, content):
        self.update_regions()
        if line_index <= len(self.regions):
            if line_index < len(self.regions):
                pos = self.regions[line_index].begin()
            else:
                pos = self.view.size()
            self.view.insert(self.edit, pos, content+"\n")
            return True
        return False

    def delete_line(self, line_index):
        self.update_regions()
        if line_index < len(self.regions):
            self.view.erase(self.edit, self.view.full_line(self.regions[line_index]))
            return True
        return False

class GsFmtCommand(sublime_plugin.TextCommand):
    def run(self, edit):
        scopes = self.view.scope_name(0).split()
        if 'source.go' not in scopes:
            return

        region = sublime.Region(0, self.view.size())
        src = self.view.substr(region)

        args = [gs.setting("gofmt_cmd", "gofmt"), "-d"]
        diff, err = gs.runcmd(args, src)
        if err:
            fn = self.view.file_name()
            err = err.replace('<standard input>', fn)
            def report_error():
                sublime.status_message('GsFmt: File %s contains errors' % fn)
            sublime.set_timeout(report_error, 0)
        elif diff:
            err = ''
            try:
                edit = self.view.begin_edit()
                ed = SublimeEditor(self.view, edit)
                err = thatcher.patch(ed, diff)
            except Exception as e:
                err = "%s\n\n%s" % (err, e)
            finally:
                self.view.end_edit(edit)
            if err:
                def report_err():
                    self.view.run_command('undo')
                    sublime.status_message("GsFmt: Could not patch the buffer: %s" % err)
                sublime.set_timeout(report_err, 0)
