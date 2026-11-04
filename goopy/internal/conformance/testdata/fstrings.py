"""Fixture: f-string bodies that a brace counter gets wrong."""
a = b = c = 1

raw_escaped_brace = rf'(\{{%)(-?\s*)({a})'
raw_field = rf'\{a}'
nested_same_quote = f"{f"{f"{a}"}"}"
brace_in_nested = f"{'{'}" f"{'}'}"
quote_in_spec = f"{a:'>10}" f'{a:">10}'
nested_spec = f'{a:{b}.{c}}'
time_spec = f'{a:%H:%M}'
dict_in_field = f'{ {1: 2}[1] }'
lambda_in_field = f'{(lambda x: 1)(2)}'
across_lines = f'''{
    a
}'''
comment_in_field = f"{
    a  # a comment with a } and a " quote
}"


def after_fstrings(): pass


class C:
    pattern = [(rf'(\{{%)({a})', a), (r'\{', a)]

    def m(self): pass


def after_all(): pass
