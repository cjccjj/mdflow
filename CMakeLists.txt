cmake_minimum_required(VERSION 3.12)

file(STRINGS VERSION MDFLOW_VERSION)
project(MDFLOW
    VERSION "${MDFLOW_VERSION}"
    HOMEPAGE_URL "https://github.com/cjccjj/mdflow"
    LANGUAGES C
)

option(BUILD_MDFLOW_EXECUTABLE "Whether to compile the mdflow executable" ON)
option(BUILD_SHARED_LIBS "Build using shared libraries" ON)

set(CMAKE_CONFIGURATION_TYPES Debug Release RelWithDebInfo MinSizeRel)
if(NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE $ENV{CMAKE_BUILD_TYPE})

    if(NOT CMAKE_BUILD_TYPE)
        set(CMAKE_BUILD_TYPE "Release")
    endif()
endif()


if(CMAKE_C_COMPILER_ID MATCHES GNU|Clang)
    add_compile_options(-Wall -Wextra -Wshadow)
    add_compile_options(-Wdeclaration-after-statement)
endif()

include(GNUInstallDirs)

add_subdirectory(src)
if (BUILD_MDFLOW_EXECUTABLE)
    add_subdirectory(mdflow)
endif ()
