# Build rules for mdflow command line utility

find_package(Threads REQUIRED)

add_executable(mdflow mdflow.c typewriter.c)
target_link_libraries(mdflow PRIVATE mdflowlib Threads::Threads)
target_compile_definitions(mdflow PRIVATE
    MDFLOW_VERSION_MAJOR=${PROJECT_VERSION_MAJOR}
    MDFLOW_VERSION_MINOR=${PROJECT_VERSION_MINOR}
    MDFLOW_VERSION_PATCH=${PROJECT_VERSION_PATCH}
)


# Install rules

install(
    TARGETS mdflow
    RUNTIME DESTINATION ${CMAKE_INSTALL_BINDIR}
)
#install(FILES "mdflow.1" DESTINATION "${CMAKE_INSTALL_MANDIR}/man1")
