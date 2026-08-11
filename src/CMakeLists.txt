# Internal targets (not installed) — used by test executables.

add_library(md4cs-batch md4cs.c md4cs.h)
target_include_directories(md4cs-batch PUBLIC "${CMAKE_CURRENT_SOURCE_DIR}")
target_compile_definitions(md4cs-batch PRIVATE "$<$<CONFIG:Debug>:DEBUG>")


add_library(md4cs md4cs.c md4cs.h)
target_include_directories(md4cs PUBLIC "${CMAKE_CURRENT_SOURCE_DIR}")
target_compile_definitions(md4cs PRIVATE MD4C_STREAMING "$<$<CONFIG:Debug>:DEBUG>")


add_library(md4cs-ansi md4cs-ansi.c md4cs-ansi.h entity.c entity.h highlight.c highlight.h html.c html.h)
target_include_directories(md4cs-ansi PUBLIC "${CMAKE_CURRENT_SOURCE_DIR}")
target_compile_definitions(md4cs-ansi PRIVATE MD4C_STREAMING "$<$<CONFIG:Debug>:DEBUG>")
target_link_libraries(md4cs-ansi PUBLIC md4cs)


# Public library — single installed artifact.

add_library(mdflowlib STATIC
    mdflow.c mdflow.h
    md4cs-ansi.c md4cs-ansi.h
    entity.c entity.h
    highlight.c highlight.h
    html.c html.h
    md4cs.c md4cs.h
)
target_include_directories(mdflowlib PUBLIC
    "$<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}>"
    "$<INSTALL_INTERFACE:${CMAKE_INSTALL_INCLUDEDIR}>"
)
target_compile_definitions(mdflowlib PRIVATE MD4C_STREAMING "$<$<CONFIG:Debug>:DEBUG>")
set_target_properties(mdflowlib PROPERTIES
    OUTPUT_NAME "mdflow"
    VERSION ${PROJECT_VERSION}
    SOVERSION ${PROJECT_VERSION_MAJOR}
    PUBLIC_HEADER "mdflow.h"
)


# Install rules

install(
    TARGETS mdflowlib
    EXPORT mdflowConfig
    ARCHIVE DESTINATION ${CMAKE_INSTALL_LIBDIR}
    LIBRARY DESTINATION ${CMAKE_INSTALL_LIBDIR}
    RUNTIME DESTINATION ${CMAKE_INSTALL_BINDIR}
    PUBLIC_HEADER DESTINATION ${CMAKE_INSTALL_INCLUDEDIR}
)

install(EXPORT mdflowConfig DESTINATION ${CMAKE_INSTALL_LIBDIR}/cmake/mdflow/ NAMESPACE mdflow::)
