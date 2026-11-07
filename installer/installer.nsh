; MyGo Launcher's installer hooks. An installed copy keeps its data in
; %LOCALAPPDATA%\MyGo Launcher (see src/data_root_windows.go); MyGo's
; uninstaller removes $INSTDIR only, so the prompt below is what deletes it.

LangString MYGO_DELETE_DATA ${LANG_ENGLISH} "Delete the launcher's data too? Choose No to keep its pages, items, icons and settings."
LangString MYGO_DELETE_DATA ${LANG_SIMPCHINESE} "同时删除启动器的数据？选择「否」保留页面、条目、图标和设置。"
LangString MYGO_MOVE_FAILED ${LANG_ENGLISH} "The launcher's old data could not be moved to your user data folder. The uninstall stopped so the data is not lost."
LangString MYGO_MOVE_FAILED ${LANG_SIMPCHINESE} "未能把启动器的旧数据搬到用户数据目录。为避免数据丢失，卸载已停止。"

!macro customUnInstall
  ; Older versions kept mygo-launcher-data in $INSTDIR. Move whatever is
  ; left there into the user's folder before the folder goes, so both
  ; answers below keep their meaning.
  IfFileExists "$INSTDIR\mygo-launcher-data\*.*" 0 legacy_done
    IfFileExists "$LOCALAPPDATA\MyGo Launcher\mygo-launcher-data\." legacy_done
      nsExec::Exec 'xcopy /E /I /Y "$INSTDIR\mygo-launcher-data" "$LOCALAPPDATA\MyGo Launcher\mygo-launcher-data"'
      Pop $0
      StrCmp $0 "0" legacy_done
      StrCmp $0 "1" legacy_done
      MessageBox MB_OK|MB_ICONSTOP "$(MYGO_MOVE_FAILED)"
      Abort
  legacy_done:

  IfFileExists "$LOCALAPPDATA\MyGo Launcher\." 0 done
    MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON2 "$(MYGO_DELETE_DATA)" /SD IDNO IDNO done
    RMDir /r "$LOCALAPPDATA\MyGo Launcher"
  done:
!macroend
