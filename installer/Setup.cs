using System;
using System.IO;
using System.IO.Compression;
using System.Reflection;
using System.Diagnostics;
using System.Drawing;
using System.Windows.Forms;
using Microsoft.Win32;
using System.Text;
using System.Collections.Generic;

class Setup : Form {
 const string Version = "0.3.0";
 const string RegPath = @"Software\Microsoft\Windows\CurrentVersion\Uninstall\NotSoFast";
 TextBox destination = new TextBox(); CheckBox shortcut = new CheckBox(); Button action = new Button(); Label status = new Label(); ProgressBar progress = new ProgressBar();
 static string ProgramFolder { get {return Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.Programs),"NotSoFast");} }
 [STAThread] static int Main(string[] args) {
  Application.EnableVisualStyles(); Application.SetCompatibleTextRenderingDefault(false);
  try {
#if UNINSTALL
   string root=Path.GetFullPath(AppDomain.CurrentDomain.BaseDirectory).TrimEnd(Path.DirectorySeparatorChar);
   bool silent=Array.IndexOf(args,"/silent")>=0;
   if(!silent && MessageBox.Show("Remove NotSoFast and its shortcuts? Your saved work in AppData will be kept.","Uninstall NotSoFast",MessageBoxButtons.YesNo,MessageBoxIcon.Question)!=DialogResult.Yes) return 0;
   Uninstall(root);
   if(!silent) MessageBox.Show("NotSoFast was removed. Your saved work has been kept.","NotSoFast");
#else
   if(Array.IndexOf(args,"/silent")>=0) {
    string target=Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),"Programs","NotSoFast");
    foreach(string arg in args) if(arg.StartsWith("/dir=",StringComparison.OrdinalIgnoreCase)) target=arg.Substring(5);
    Install(target,false,null);
   } else Application.Run(new Setup());
#endif
   return 0;
  } catch(Exception e) { try {File.AppendAllText(Path.Combine(Path.GetTempPath(),"NotSoFast-Setup.log"),DateTime.UtcNow.ToString("O")+" "+e.ToString()+Environment.NewLine);}catch{} if(Array.IndexOf(args,"/silent")<0) MessageBox.Show(e.Message,"NotSoFast setup",MessageBoxButtons.OK,MessageBoxIcon.Error); else Console.Error.WriteLine(e.Message); return 1; }
 }
 public Setup(){
  Text="NotSoFast — Setup";ClientSize=new Size(660,510);FormBorderStyle=FormBorderStyle.FixedDialog;MaximizeBox=false;StartPosition=FormStartPosition.CenterScreen;BackColor=Color.FromArgb(17,23,21);ForeColor=Color.FromArgb(240,245,235);Font=new Font("Segoe UI",10);
  var brand=new Label{Text="N↗  NOTSOFAST",Location=new Point(38,28),Size=new Size(550,28),ForeColor=Color.FromArgb(213,250,85),Font=new Font("Segoe UI",12,FontStyle.Bold)};
  var title=new Label{Text="Your next move, verified.",Location=new Point(35,82),Size=new Size(600,50),Font=new Font("Segoe UI",27,FontStyle.Bold)};
  var intro=new Label{Text="A private workspace for evidence, guarded actions and AI experiments.\nDesktop + browser. Everything included. No terminal required.",Location=new Point(39,146),Size=new Size(580,60)};
  var label=new Label{Text="Install for this Windows account",Location=new Point(39,225),AutoSize=true};
  destination.SetBounds(39,254,470,30);destination.Text=Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),"Programs","NotSoFast");
  var browse=new Button{Text="Browse…",Location=new Point(521,252),Size=new Size(96,32)};browse.Click+=(s,e)=>{using(var picker=new FolderBrowserDialog()){picker.Description="Choose a dedicated NotSoFast installation folder";if(picker.ShowDialog()==DialogResult.OK)destination.Text=Path.Combine(picker.SelectedPath,"NotSoFast");}};
  shortcut.Text="Add a desktop shortcut";shortcut.SetBounds(39,305,350,28);shortcut.Checked=true;
  var notes=new Label{Text="Includes bundled Git and GitHub sign-in tools.\nAn uninstaller is included. Updates keep your saved work.",Location=new Point(39,349),Size=new Size(560,50),ForeColor=Color.FromArgb(177,190,176)};
  progress.SetBounds(39,407,578,6);progress.Style=ProgressBarStyle.Continuous;
  status.SetBounds(39,439,395,40);status.Text="Ready to install · Windows x64 · "+Version;
  action.SetBounds(459,435,158,40);action.Text="Install NotSoFast";action.BackColor=Color.FromArgb(213,250,85);action.ForeColor=Color.FromArgb(20,30,15);action.FlatStyle=FlatStyle.Flat;
  action.Click+=(s,e)=>{action.Enabled=false;destination.Enabled=false;browse.Enabled=false;try{status.Text="Installing files and shortcuts…";Refresh();Install(destination.Text,shortcut.Checked,n=>{progress.Value=n;Refresh();});status.Text="Installed. Your workspace is ready.";action.Text="Open NotSoFast";action.Enabled=true;action.Click+=(s2,e2)=>{};action.Tag="installed";Process.Start(Path.Combine(destination.Text,"NotSoFast.exe"));Close();}catch(Exception ex){MessageBox.Show(ex.Message,"Setup could not finish",MessageBoxButtons.OK,MessageBoxIcon.Error);status.Text="Close the app if it is running, then retry.";action.Enabled=true;destination.Enabled=true;browse.Enabled=true;}};
  Controls.AddRange(new Control[]{brand,title,intro,label,destination,browse,shortcut,notes,progress,status,action});
 }
 static void CheckAncestors(string path){for(string p=Path.GetFullPath(path);!string.IsNullOrEmpty(p);p=Path.GetDirectoryName(p)){if((Directory.Exists(p)||File.Exists(p))&&(File.GetAttributes(p)&FileAttributes.ReparsePoint)!=0)throw new IOException("Installation paths must not contain symbolic links or junctions.");}}
 static void CheckTree(string root){CheckAncestors(root);if(!Directory.Exists(root))return;var pending=new Stack<string>();pending.Push(root);while(pending.Count>0){foreach(string entry in Directory.GetFileSystemEntries(pending.Pop())){var attributes=File.GetAttributes(entry);if((attributes&FileAttributes.ReparsePoint)!=0)throw new IOException("Installation contains a symbolic link or junction. Remove the link before updating or uninstalling.");if((attributes&FileAttributes.Directory)!=0)pending.Push(entry);}}}
 static string FullChild(string root,string relative){string full=Path.GetFullPath(Path.Combine(root,relative));if(!full.StartsWith(root.TrimEnd(Path.DirectorySeparatorChar)+Path.DirectorySeparatorChar,StringComparison.OrdinalIgnoreCase))throw new IOException("Invalid installation path.");CheckAncestors(full);return full;}
 static bool Running(string root){foreach(var p in Process.GetProcesses()){try{string file=p.MainModule.FileName;if(file.StartsWith(root+Path.DirectorySeparatorChar,StringComparison.OrdinalIgnoreCase)&&p.Id!=Process.GetCurrentProcess().Id)return true;}catch{}finally{p.Dispose();}}return false;}
 static void Link(string name,string target){Type t=Type.GetTypeFromProgID("WScript.Shell");object shell=Activator.CreateInstance(t);object link=t.InvokeMember("CreateShortcut",BindingFlags.InvokeMethod,null,shell,new object[]{name});try{var lt=link.GetType();lt.InvokeMember("TargetPath",BindingFlags.SetProperty,null,link,new object[]{target});lt.InvokeMember("WorkingDirectory",BindingFlags.SetProperty,null,link,new object[]{Path.GetDirectoryName(target)});lt.InvokeMember("Save",BindingFlags.InvokeMethod,null,link,null);}finally{System.Runtime.InteropServices.Marshal.FinalReleaseComObject(link);System.Runtime.InteropServices.Marshal.FinalReleaseComObject(shell);}}
 static void RemoveOwnedLink(string name,string root){if(!File.Exists(name))return;Type t=Type.GetTypeFromProgID("WScript.Shell");object shell=Activator.CreateInstance(t);object link=t.InvokeMember("CreateShortcut",BindingFlags.InvokeMethod,null,shell,new object[]{name});try{string target=(string)link.GetType().InvokeMember("TargetPath",BindingFlags.GetProperty,null,link,null);if(!string.IsNullOrEmpty(target)&&Path.GetFullPath(target).StartsWith(root+Path.DirectorySeparatorChar,StringComparison.OrdinalIgnoreCase))File.Delete(name);}finally{System.Runtime.InteropServices.Marshal.FinalReleaseComObject(link);System.Runtime.InteropServices.Marshal.FinalReleaseComObject(shell);}}
 static void Install(string path,bool desktop,Action<int> update){
#if !UNINSTALL
  string root=Path.GetFullPath(path).TrimEnd(Path.DirectorySeparatorChar);
  CheckTree(root);
  if(root==Path.GetPathRoot(root).TrimEnd(Path.DirectorySeparatorChar)||root.Length<10)throw new IOException("Choose a dedicated application folder.");
  if(Directory.Exists(root)&&Directory.GetFileSystemEntries(root).Length>0&&!File.Exists(Path.Combine(root,"installed-files.txt")))throw new IOException("Choose an empty folder or an existing NotSoFast installation.");
  if(Running(root))throw new IOException("Quit NotSoFast using the app's Quit button before installing this update.");
  string stage=root+".setup-"+Guid.NewGuid().ToString("N"), backup=root+".previous-"+Guid.NewGuid().ToString("N");bool replaced=false;
  Directory.CreateDirectory(stage);
  try{
   using(var stream=Assembly.GetExecutingAssembly().GetManifestResourceStream("payload.zip"))using(var zip=new ZipArchive(stream,ZipArchiveMode.Read)){
    int n=0;foreach(var entry in zip.Entries){string name=entry.FullName.Replace('\\','/');if(!name.StartsWith("NotSoFast/"))throw new IOException("Invalid setup payload.");name=name.Substring(10);if(name.Length==0)continue;string dest=FullChild(stage,name);if(name.EndsWith("/")){Directory.CreateDirectory(dest);continue;}Directory.CreateDirectory(Path.GetDirectoryName(dest));using(var input=entry.Open())using(var output=File.Create(dest))input.CopyTo(output);if(update!=null)update(++n*85/zip.Entries.Count);}
   }
   using(var input=Assembly.GetExecutingAssembly().GetManifestResourceStream("uninstall.exe"))using(var output=File.Create(Path.Combine(stage,"Uninstall.exe")))input.CopyTo(output);
   var names=new List<string>();foreach(string f in Directory.GetFiles(stage,"*",SearchOption.AllDirectories))names.Add(f.Substring(stage.Length+1));File.WriteAllLines(Path.Combine(stage,"installed-files.txt"),names.ToArray());
   if(Directory.Exists(root))Directory.Move(root,backup);Directory.Move(stage,root);replaced=true;
   Directory.CreateDirectory(ProgramFolder);Link(Path.Combine(ProgramFolder,"NotSoFast.lnk"),Path.Combine(root,"NotSoFast.exe"));Link(Path.Combine(ProgramFolder,"NotSoFast Web.lnk"),Path.Combine(root,"NotSoFast-Web.exe"));Link(Path.Combine(ProgramFolder,"Uninstall.lnk"),Path.Combine(root,"Uninstall.exe"));
   if(desktop)Link(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory),"NotSoFast.lnk"),Path.Combine(root,"NotSoFast.exe"));
   using(var key=Registry.CurrentUser.CreateSubKey(RegPath)){key.SetValue("DisplayName","NotSoFast");key.SetValue("DisplayVersion",Version);key.SetValue("Publisher","NotSoFast contributors");key.SetValue("InstallLocation",root);key.SetValue("UninstallString","\""+Path.Combine(root,"Uninstall.exe")+"\"");key.SetValue("DisplayIcon",Path.Combine(root,"NotSoFast.exe"));key.SetValue("NoModify",1);key.SetValue("NoRepair",1);}
   if(update!=null)update(100);
   // Keep an old installation backup if it contains user-added files.
   if(Directory.Exists(backup))RemoveListedFiles(backup,false);
  }catch{if(!replaced&&Directory.Exists(backup)&&!Directory.Exists(root))Directory.Move(backup,root);throw;}
  finally{if(Directory.Exists(stage))Directory.Delete(stage,true);}
#endif
 }
 static void RemoveListedFiles(string root,bool skipSelf){CheckTree(root);string manifest=Path.Combine(root,"installed-files.txt");if(!File.Exists(manifest))throw new IOException("Installation manifest missing. Refusing to remove files.");foreach(string relative in File.ReadAllLines(manifest)){string f=FullChild(root,relative);if(skipSelf&&string.Equals(f,Assembly.GetExecutingAssembly().Location,StringComparison.OrdinalIgnoreCase))continue;if(File.Exists(f))File.Delete(f);}File.Delete(manifest);string[] dirs=Directory.GetDirectories(root,"*",SearchOption.AllDirectories);Array.Sort(dirs,(a,b)=>b.Length.CompareTo(a.Length));foreach(string d in dirs)if(Directory.GetFileSystemEntries(d).Length==0)Directory.Delete(d);if(Directory.GetFileSystemEntries(root).Length==0)Directory.Delete(root);}
 static void Uninstall(string root){
  if(Running(root))throw new IOException("Quit NotSoFast before uninstalling. Your saved work will be kept.");
  RemoveListedFiles(root,true);
  foreach(string n in new[]{"NotSoFast.lnk","NotSoFast Web.lnk","Uninstall.lnk"}){string link=Path.Combine(ProgramFolder,n);RemoveOwnedLink(link,root);}
  if(Directory.Exists(ProgramFolder)&&Directory.GetFileSystemEntries(ProgramFolder).Length==0)Directory.Delete(ProgramFolder);
  string desktop=Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory),"NotSoFast.lnk");RemoveOwnedLink(desktop,root);
  bool ownsRegistration=false;using(var key=Registry.CurrentUser.OpenSubKey(RegPath)){ownsRegistration=key!=null&&string.Equals(key.GetValue("InstallLocation") as string,root,StringComparison.OrdinalIgnoreCase);}
  if(ownsRegistration)Registry.CurrentUser.DeleteSubKeyTree(RegPath,false);
  string script="Start-Sleep -Seconds 2; Remove-Item -LiteralPath '"+Assembly.GetExecutingAssembly().Location.Replace("'","''")+"' -Force; if ((Get-ChildItem -LiteralPath '"+root.Replace("'","''")+"' -Force | Measure-Object).Count -eq 0) { Remove-Item -LiteralPath '"+root.Replace("'","''")+"' }";
  var psi=new ProcessStartInfo(Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.System),@"WindowsPowerShell\v1.0\powershell.exe"),"-NoProfile -NonInteractive -EncodedCommand "+Convert.ToBase64String(Encoding.Unicode.GetBytes(script))){UseShellExecute=false,CreateNoWindow=true};Process.Start(psi);
 }
}
