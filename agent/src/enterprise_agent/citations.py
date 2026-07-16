from __future__ import annotations

import re

from enterprise_agent.models import CitationValidation, ExternalLink, RetrievalChunk

KNOWLEDGE_REFERENCE = re.compile(r"\[引用(\d+)]")
EXTERNAL_REFERENCE = re.compile(r"\[外链(\d+)]")


def normalize_answer(answer: str) -> str:
    lines = [line.strip() for line in answer.replace("\r\n", "\n").split("\n")]
    normalized: list[str] = []
    for line in lines:
        if not line:
            if normalized and normalized[-1]:
                normalized.append("")
            continue
        normalized.append(line)
    return "\n".join(normalized).strip()


def select_referenced_sources(
    answer: str,
    chunks: list[RetrievalChunk],
    links: list[ExternalLink],
) -> tuple[str, list[RetrievalChunk], list[ExternalLink]]:
    answer = normalize_answer(answer)
    if is_no_answer(answer):
        return "无法确定", [], []

    chunk_indexes = _reference_indexes(KNOWLEDGE_REFERENCE, answer, len(chunks))
    link_indexes = _reference_indexes(EXTERNAL_REFERENCE, answer, len(links))
    answer = _remap(answer, KNOWLEDGE_REFERENCE, chunk_indexes, "引用")
    answer = _remap(answer, EXTERNAL_REFERENCE, link_indexes, "外链")
    selected_chunks = [chunks[index] for index in chunk_indexes]
    selected_links = [links[index] for index in link_indexes]
    return answer, selected_chunks, selected_links


def validate_reference_structure(
    answer: str,
    chunks: list[RetrievalChunk],
    links: list[ExternalLink],
) -> CitationValidation:
    normalized = normalize_answer(answer)
    if is_no_answer(normalized):
        return CitationValidation(supported=True)
    references = [
        *(f"引用{value}" for value in KNOWLEDGE_REFERENCE.findall(normalized)),
        *(f"外链{value}" for value in EXTERNAL_REFERENCE.findall(normalized)),
    ]
    invalid = [
        reference
        for reference in references
        if (
            reference.startswith("引用")
            and not 1 <= int(reference.removeprefix("引用")) <= len(chunks)
        )
        or (
            reference.startswith("外链")
            and not 1 <= int(reference.removeprefix("外链")) <= len(links)
        )
    ]
    if not references:
        return CitationValidation(
            supported=False,
            unsupported_claims=["回答包含结论但没有引用"],
            reason="missing_citations",
        )
    if invalid:
        return CitationValidation(
            supported=False,
            invalid_references=list(dict.fromkeys(invalid)),
            reason="invalid_reference_index",
        )
    return CitationValidation(supported=True)


def is_no_answer(answer: str) -> bool:
    normalized = answer.strip().rstrip("。.!！")
    return normalized in {"无法确定", "无法回答", "资料不足，无法确定"}


def _reference_indexes(pattern: re.Pattern[str], answer: str, limit: int) -> list[int]:
    indexes: list[int] = []
    for raw in pattern.findall(answer):
        index = int(raw) - 1
        if 0 <= index < limit and index not in indexes:
            indexes.append(index)
    return indexes


def _remap(answer: str, pattern: re.Pattern[str], indexes: list[int], label: str) -> str:
    mapping = {original + 1: current + 1 for current, original in enumerate(indexes)}

    def replace(match: re.Match[str]) -> str:
        original = int(match.group(1))
        mapped = mapping.get(original)
        return f"[{label}{mapped}]" if mapped is not None else ""

    return pattern.sub(replace, answer)
