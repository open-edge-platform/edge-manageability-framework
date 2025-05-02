#!/usr/bin/env python

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

"""
sort_charts.py
combines artifact and manifest information into datastructure that can be used
to determine which charts to build
"""

import sys
import ruamel.yaml

from ba_lib import load_yaml, load_repo_artifacts

yaml = ruamel.yaml.YAML()

# globals
repo_tags_to_build = {}

req_charts = []


def match_charts(chart_mf, chart_to_repo, artifacts):
    """match up charts in manifest with where they are built in repos"""

    for chart, repo in chart_to_repo.items():

        # most component names == chart name, but not all (web-ui and orch-ui)
        for _, cdata in chart_mf["components"].items():
            if chart == cdata["chart"]:

                req_charts.remove(chart)

                repo_chart_data = artifacts[repo]["charts"][chart]

                manifest_chart_data = cdata

                chart_tag_to_build = (
                    repo_chart_data["gitTagPrefix"] + manifest_chart_data["version"]
                )

                if "outDir" in repo_chart_data:
                    chart_tag_to_build += "|" + repo_chart_data["outDir"]

                if repo in repo_tags_to_build:
                    if chart_tag_to_build not in repo_tags_to_build[repo]["charts"]:
                        repo_tags_to_build[repo]["charts"].append(chart_tag_to_build)
                else:
                    repo_tags_to_build[repo] = {"charts": [], "images": []}
                    repo_tags_to_build[repo]["charts"].append(chart_tag_to_build)


if __name__ == "__main__":

    chart_manifest = load_yaml("scratch/manifest_charts.yaml")

    # create a list of required charts, but only ours (with oci:// url)
    for _, chart_data in chart_manifest["components"].items():
        if chart_data["repo"].startswith("oci://"):
            req_charts.append(chart_data["chart"])

    repo_artifacts, ctr, _itr = load_repo_artifacts("scratch")

    match_charts(chart_manifest, ctr, repo_artifacts)

    for rname, data in repo_tags_to_build.items():
        if data["charts"]:  # If there are charts in the repo

            # write tags into a file, one per line
            with open(f"scratch/htags_{rname}", "w", encoding="utf-8") as htagout:
                for line in data["charts"]:
                    htagout.write(f"{line}\n")

    if req_charts:  # die if missing charts if they exist
        print(f"ERROR - Missing charts: {req_charts}")
        sys.exit(1)
