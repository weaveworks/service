load("@io_bazel_rules_go//go:def.bzl", "go_path")

load("@io_bazel_rules_go//go/private:providers.bzl", "GoLibrary", "GoPath")

_MOCKGEN_TOOL = "//vendor/github.com/golang/mock/mockgen"
        
def _gomock_sh_impl(ctx):
    go_toolchain = ctx.toolchains["@io_bazel_rules_go//go:toolchain"]
    gopath = "$(pwd)/" + ctx.var["BINDIR"] + "/" + ctx.attr.gopath_dep[GoPath].gopath

    pkg_args = []
    if ctx.attr.package != '':
        pkg_args = ["-package", ctx.attr.package]
    args = pkg_args + [
        "-destination", "$(pwd)/"+ ctx.outputs.out.path,
        ctx.attr.library[GoLibrary].importpath,
        ",".join(ctx.attr.interfaces),
    ]
        
    ctx.actions.run_shell(
        outputs = [ctx.outputs.out],
        inputs = [ctx.file._mockgen] + ctx.attr.gopath_dep.files.to_list(),
        command = """
		   export GOROOT=/home/awh/cloud/src/service/bazel-service/external/go_sdk &&
           export PATH=$GOROOT/bin:$PATH &&
           export GOPATH={gopath} &&
           {mockgen} {args}
        """.format(
            gopath=gopath,
            mockgen="$(pwd)/"+ctx.file._mockgen.path,
            args = " ".join(args)
        )
    )

_gomock_sh = rule(
    _gomock_sh_impl,
    attrs = {
        "library": attr.label(
            doc = "The target the Go library is at to look for the interfaces in. When this is set, mockgen will use its reflect code to generate the mocks. source cannot also be set when this is set.",
            providers = [GoLibrary],
            mandatory = True,
        ),

        "gopath_dep": attr.label(
            doc = "The go_path label to use to create the GOPATH for the given library",
            providers=[GoPath],
            mandatory = True
        ),

        "out": attr.output(
            doc = "The new Go file to emit the generated mocks into",
            mandatory = True
        ),
    
        "interfaces": attr.string_list(
            allow_empty = False,
            doc = "The names of the Go interfaces to generate mocks for",
            mandatory = True,
        ),
        "package": attr.string(
            doc = "The name of the package the generated mocks should be in. If not specified, uses mockgen's default.",
        ),
        "_mockgen": attr.label(
            doc = "The mockgen tool to run",
            default = Label(_MOCKGEN_TOOL),
            allow_single_file = True,
            executable = True,
            cfg = "host",
        ),
    },
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)

def gomock(name, library, out, **kwargs):
    gopath_name = name + "_gomock_gopath"
    go_path(
        name = gopath_name,
        # FIXME make this _MOCKGEN_TOOL overridable like the other one. This is because of a weird runtime dep on github.com/golang/mock/mockgen/model
        deps = [library, _MOCKGEN_TOOL],
    )

    _gomock_sh(
        name = name,
        library = library,
        gopath_dep = gopath_name,
        out = out,
        **kwargs
    )

