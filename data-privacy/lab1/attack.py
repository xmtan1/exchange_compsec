"""
Reconstruction attack implementation as described by Nissim and Dinur.
"""

from itertools import chain, combinations
from typing import List, Tuple

from db import *

# A candidate is a guess on the `hiv` conditions of all the patients.
Candidate = List[HIV]

# A candidate with (public) patient names attached.
CandidateWithNames = List[Tuple[Name, HIV]]

# The result of our queries as a `float`, since we are releasing sums of `hiv` conditions with noise.
ResultQuery = float

def comb(k: int, ns: List[int]) -> List[List[int]]:
  """Generates the combination of elements in `ns` by taking groups of size `k`."""
  return list(map(list, combinations(ns, k)))

def indexes(db_size: int) -> List[List[Index]]:
  """Generates all possible combinations of indexes for a database of given `db_size`."""
  pass # TODO: Implement this function

def all_sums(db: DB, noise: Noise) -> List[ResultQuery]:
  """Performs the noisy sums of all combinations of indexes for a given database `db`. It calls `add`."""
  pass # TODO: Implement this function

def sum_indexes(candidate: Candidate, idx: List[Index]) -> ResultQuery:
  """Given a candidate and some indexes `idx`, it performs the sum of the conditions (without noise)."""
  pass # TODO: Implement this function

def all_sums_no_noise(candidate: Candidate) -> List[ResultQuery]:
  """Given a candidate, it performs the sums (without noise) of all combinations of indexes."""
  pass # TODO: Implement this function

def generate_candidates(db: DB) -> List[Candidate]:
  """Given a database `db`, it generates all the possible candidates."""
  pass # TODO: Implement this function

def fit(noise_mag: Noise, results: List[ResultQuery], candidate: Candidate) -> bool:
  """
  This function will determine if exists a non-noisy sum on the candidate and a
  corresponding noisy sum in `results` whose distance is greater than `noise_mag`.
  """
  pass # TODO: Implement this function

def find_candidates(db: DB, noise: Noise) -> List[Candidate]:
  """Finds candidates whose non-noisy sums "fit" the noisy ones."""
  pass # TODO: Implement this function

def attack(db: DB, noise: Noise) -> List[CandidateWithNames]:
  """Guess the conditions of all patients in the dataset."""
  pass # TODO: Implement this function
