// Type declarations for Monaco Editor ESM sub-path imports.
// These side-effect imports register languages/features but have no .d.ts files.

declare module 'monaco-editor/esm/vs/editor/editor.all';

declare module 'monaco-editor/esm/vs/editor/editor.api' {
  export * from 'monaco-editor';
}

// Language services
declare module 'monaco-editor/esm/vs/language/typescript/monaco.contribution';
declare module 'monaco-editor/esm/vs/language/css/monaco.contribution';
declare module 'monaco-editor/esm/vs/language/json/monaco.contribution';
declare module 'monaco-editor/esm/vs/language/html/monaco.contribution';

// All basic language syntax contributions
declare module 'monaco-editor/esm/vs/basic-languages/abap/abap.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/apex/apex.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/azcli/azcli.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/bat/bat.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/bicep/bicep.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/cameligo/cameligo.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/clojure/clojure.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/coffee/coffee.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/cpp/cpp.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/csharp/csharp.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/csp/csp.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/css/css.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/cypher/cypher.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/dart/dart.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/dockerfile/dockerfile.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/ecl/ecl.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/elixir/elixir.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/flow9/flow9.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/freemarker2/freemarker2.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/fsharp/fsharp.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/go/go.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/graphql/graphql.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/handlebars/handlebars.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/hcl/hcl.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/html/html.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/ini/ini.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/java/java.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/javascript/javascript.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/julia/julia.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/kotlin/kotlin.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/less/less.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/lexon/lexon.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/liquid/liquid.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/lua/lua.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/m3/m3.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/markdown/markdown.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/mdx/mdx.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/mips/mips.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/msdax/msdax.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/mysql/mysql.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/objective-c/objective-c.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/pascal/pascal.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/pascaligo/pascaligo.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/perl/perl.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/pgsql/pgsql.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/php/php.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/pla/pla.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/postiats/postiats.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/powerquery/powerquery.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/powershell/powershell.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/protobuf/protobuf.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/pug/pug.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/python/python.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/qsharp/qsharp.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/r/r.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/razor/razor.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/redis/redis.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/redshift/redshift.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/restructuredtext/restructuredtext.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/ruby/ruby.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/rust/rust.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/sb/sb.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/scala/scala.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/scheme/scheme.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/scss/scss.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/shell/shell.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/solidity/solidity.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/sophia/sophia.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/sparql/sparql.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/sql/sql.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/st/st.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/swift/swift.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/systemverilog/systemverilog.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/tcl/tcl.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/twig/twig.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/typescript/typescript.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/typespec/typespec.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/vb/vb.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/wgsl/wgsl.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/xml/xml.contribution';
declare module 'monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution';

// Workers
declare module 'monaco-editor/esm/vs/editor/editor.worker?worker' {
  const WorkerConstructor: new () => Worker;
  export default WorkerConstructor;
}
declare module 'monaco-editor/esm/vs/language/typescript/ts.worker?worker' {
  const WorkerConstructor: new () => Worker;
  export default WorkerConstructor;
}
declare module 'monaco-editor/esm/vs/language/css/css.worker?worker' {
  const WorkerConstructor: new () => Worker;
  export default WorkerConstructor;
}
declare module 'monaco-editor/esm/vs/language/html/html.worker?worker' {
  const WorkerConstructor: new () => Worker;
  export default WorkerConstructor;
}
declare module 'monaco-editor/esm/vs/language/json/json.worker?worker' {
  const WorkerConstructor: new () => Worker;
  export default WorkerConstructor;
}
