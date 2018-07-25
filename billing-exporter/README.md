## Why is this exporter written in python and not GO?

We have found that while investigating billing issues, it helps to use a language like python which is more suited to exploratory analysis than GO.

We also found that we were lacking a library of python code to query our billing system, which took some time to establish.

This exporter is an place where such a library of python billing code can be put to some use, during the (hopefully) long seasons in which we don't need to manually verify the behaviour of our billing system.