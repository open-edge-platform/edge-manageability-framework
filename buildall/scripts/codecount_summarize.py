#!/usr/bin/env python3

# codecount_summarize.py

# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

"""
Create CSV summarization of code counts from scc tool .json output
"""

import csv
import json
import re
import sys
import pathlib
import argparse

repos_cc = {}
repos_cc_reorg = {}
all_langs = {}

repo_groups = []


def load_scc(scc_filename):
    """
    load and parse the scc codecount json
    """

    with open(scc_filename, "r", encoding="utf-8") as cc_in:
        try:
            jdata = json.load(cc_in)

            add_groups_and_langs(str(scc_filename), jdata)

        except json.decoder.JSONDecodeError:
            print(f"ERROR: Can't load {scc_filename}")
            sys.exit(1)


def add_groups_and_langs(scc_filename, cc_data):
    """
    add to repo_groups and all_langs with codecount data
    """

    # strip off dirname, prefix, and ending .json
    repo_name = re.split(".*/scc_([a-z0-9-_.]+).json", scc_filename)[1]

    # group by prefix
    re_pre = re.split("([a-z0-9.]+)-.*", repo_name)
    if len(re_pre) == 1:  # if no dash in name
        repo_group = re_pre[0]
    else:
        repo_group = re_pre[1]

    if repo_group not in repo_groups:
        repo_groups.append(repo_group)

    repos_cc[repo_name] = cc_data

    # dict with language as key and dict of counts as value, rather than list
    # of counts
    lang_reorg = {}

    for lang in cc_data["languageSummary"]:

        l_name = lang["Name"]

        lang_reorg[l_name] = lang

        if l_name not in all_langs:
            all_langs[l_name] = 1
        else:
            all_langs[l_name] += 1

    repos_cc_reorg[repo_name] = lang_reorg


def generate_langs_csv(out_csv_filename):
    """
    create csv with per-language counts
    """

    # sorted list of language tuples, second item is # of repos
    lang_s = sorted(all_langs.items(), key=lambda a: a[1])

    # column order is languages by most to least frequent use
    lang_header = list(map(lambda a: a[0], reversed(lang_s)))

    # init list of totals per lang
    lang_totals = {l: 0 for l in lang_header}

    with open(out_csv_filename, "w", encoding="utf-8") as csvfile:
        cw = csv.writer(csvfile, quoting=csv.QUOTE_NONNUMERIC, lineterminator="\n")

        cw.writerow(["Repo"] + lang_header)

        # write repo rows
        for repo, langcounts in sorted(repos_cc_reorg.items()):

            repo_row = [repo]

            for l_col in lang_header:

                if l_col in langcounts:
                    lines = langcounts[l_col]["Code"]
                    repo_row.append(lines)

                    lang_totals[l_col] += lines
                else:
                    repo_row.append(0)

            cw.writerow(repo_row)

        # write total row
        total_row = ["Total"]
        for t_col in lang_header:
            total_row.append(lang_totals[t_col])

        cw.writerow(total_row)


def generate_totals_csv(out_csv_filename, repo_data):
    """
    iterate over repo_data, and generate one line per repo in CSV file,
    grouping types of data, and generate a total of all repos
    """

    # many accumulating total variables are required
    # pylint: disable=too-many-locals

    t_code = 0
    t_gen = 0
    t_build = 0
    t_data = 0
    t_comment = 0
    t_other = 0

    with open(out_csv_filename, "w", encoding="utf-8") as csvfile:
        cw = csv.writer(csvfile, quoting=csv.QUOTE_NONNUMERIC, lineterminator="\n")

        # header row
        cw.writerow(
            [
                "Repo",
                "Code Lines",
                "Generated Lines",
                "Build",
                "Data",
                "Comments/Docs",
                "Other",
            ]
        )

        for rname, rdata in repo_data.items():

            r_code = 0
            r_gen = 0
            r_build = 0
            r_data = 0
            r_comment = 0
            r_other = 0

            # group by file type and type of code
            for lname, ldata in rdata.items():

                if re.match(".*(gen).*", lname):
                    r_gen += ldata["Code"]
                elif lname in [
                    "Autoconf",
                    "CMake",
                    "Docker ignore",
                    "Dockerfile",
                    "Groovy",
                    "Makefile",
                ]:
                    r_build += ldata["Code"]

                elif lname in [
                    "Assembly",
                    "BASH",
                    "C Header",
                    "C",
                    "C++ Header",
                    "C++",
                    "CSS",
                    "Go",
                    "GraphQL",
                    "HTML",
                    "Java",
                    "JavaScript (min)",
                    "JavaScript",
                    "Jupyter (min)",
                    "Jupyter",
                    "LESS (gen)",
                    "Lua",
                    "PHP",
                    "Perl",
                    "Protocol Buffers",
                    "Python",
                    "Robot Framework (min)",
                    "Robot Framework",
                    "Ruby",
                    "Rust",
                    "Sass",
                    "Shell",
                    "Systemd",
                    "TypeScript (min)",
                    "TypeScript Typings",
                    "TypeScript",
                ]:
                    r_code += ldata["Code"]

                elif lname in [
                    "CSV",
                    "CloudFormation (YAML)",
                    "Go Template",
                    "INI",
                    "JSON (min)",
                    "JSON",
                    "Patch",
                    "Smarty Template",
                    "TOML",
                    "Terraform",
                    "XML",
                    "XML Schema",
                    "YAML",
                ]:
                    r_data += ldata["Code"]

                elif lname in [
                    "DOT",
                    "Document Type Definition",
                    "License",
                    "Markdown",
                    "Plain Text",
                    "ReStructuredText",
                    "SVG (min)",
                    "SVG",
                    "TeX",
                ]:
                    r_comment += ldata["Code"]

                else:
                    r_other += ldata["Code"]

                r_comment += ldata["Comment"]

            cw.writerow([rname, r_code, r_gen, r_build, r_data, r_comment, r_other])

            # accumulate for whole group
            t_code += r_code
            t_gen += r_gen
            t_build += r_build
            t_data += r_data
            t_comment += r_comment
            t_other += r_other

    return [t_code, t_gen, t_build, t_data, t_comment, t_other]


if __name__ == "__main__":

    ap = argparse.ArgumentParser()
    ap.add_argument("output_dir")
    args = ap.parse_args()

    scc_path = pathlib.Path(f"{args.output_dir}/")

    for scc_file in list(scc_path.glob("scc*.json")):
        load_scc(scc_file)

    generate_langs_csv(f"{args.output_dir}/codecount_langs.csv")

    group_totals = {}

    for group in repo_groups:
        group_repos = {}

        # make a dict with only group repos out by repo name
        for name, data in repos_cc_reorg.items():
            if re.match(group, name):
                group_repos[name] = data

        if group_repos:
            group_totals[group] = generate_totals_csv(
                f"{args.output_dir}/codecount_{group}_group.csv", group_repos
            )

    with open(
        f"{args.output_dir}/codecount_alltotals.csv", "w", encoding="utf-8"
    ) as tcsvfile:
        tcw = csv.writer(tcsvfile, quoting=csv.QUOTE_NONNUMERIC, lineterminator="\n")

        tcw.writerow(
            [
                "Group",
                "Code Lines",
                "Generated Lines",
                "Build",
                "Data",
                "Comments/Docs",
                "Other",
            ]
        )

        for gname, gtotal in group_totals.items():
            tcw.writerow([gname] + gtotal)
