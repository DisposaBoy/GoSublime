import sublime, sublime_plugin
import gscommon as gs
from os.path import basename

class GoFmt(sublime_plugin.EventListener):
    def on_pre_save(self, view):
        scopes = view.scope_name(0).split()
        should_run = gs.setting("run_gofmt_on_save", False)
        if 'source.go' not in scopes or should_run is not True:
            return
        
        region = sublime.Region(0, view.size())
        view_src = view.substr(region)

        args = [gs.setting("gofmt_cmd", "gofmt")]
        src, err = gs.runcmd(args, view_src)    
        if err:
            fn = basename(view.file_name())
            err = err.replace('<standard input>', fn)
            sublime.error_message(err)
        elif src.strip() and src != view_src:
            vr = view.visible_region()
            edit = view.begin_edit()
            view.replace(edit, region,  src)
            view.end_edit(edit)
            view.show_at_center(vr)
