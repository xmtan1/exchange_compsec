from csv import reader as csv_reader
from dataclasses import dataclass
from random import uniform
from typing import List

# Names are modeled as strings.
Name = str

# HIV condition is modeled as a `float` for simplicity.
# Value 0 means the patient does not have HIV, and 1 otherwise.
HIV = float

@dataclass
class Row:
  """A Row is a register with two fields: the name of the patient and his/her hiv condition"""
  name: Name
  hiv: HIV

@dataclass
class DB:
  """A database is simply a list of rows of type Row"""
  secret_rows: List[Row]
  # NOTE: The `secret_rows` field is considered secret data that your (hypothetical) attack does not have access to.
  # Do _not_ refer to it in your code! We will grep :)

# An Index is a row in the database.
Index = int

# We model noise as a `float` value.
Noise = float

def to_row(fields: List[str]) -> Row:
  """Convert a list of strings to a Row."""

  if len(fields) != 2:
    raise ValueError("toRow: Incorrect format")

  return Row(name=fields[0], hiv=float(fields[1]))

def to_rows(csv_data: List[List[str]]) -> List[Row]:
  """Convert CSV data to a list of Rows."""
  return list(map(to_row, csv_data))

def db_size(db: DB) -> int:
  """Provides the number of rows in the dataset."""
  return len(db.secret_rows)

def names(db: DB) -> List[Name]:
  """Provides the names of patients in the same order as they appear in the dataset."""
  return [row.name for row in db.secret_rows]

def add(db: DB, noise: Noise, indices: List[Index]) -> float:
  """Adding elements at the position given given by the `List[Index]` with a given noise."""
  s = sum(db.secret_rows[i].hiv for i in indices)
  noise = uniform(-noise, noise)
  return s + noise

def import_csv(path: str) -> List[List[str]]:
  """Auxiliary function for handling CSV files."""
  with open(path, 'r', newline='') as csv_file:
    reader = csv_reader(csv_file)
    rows = list(reader)
    return rows[1:] # Remove header (first row)

def load_db(path: str) -> DB:
  """Load a dataset from a file."""
  return DB(to_rows(import_csv(path)))
