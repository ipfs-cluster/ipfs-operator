Hacking
===================================

Have you have found a bug in the operator or do you have an additional feature? That's great! Here are some tips.


Running your own images.

If you have edited the code, you would like to install it in your own cluster, you will first need an account on an image host like dockerhub. Once you have this, you can build, push, and install the image using the **IMG** environment variable
::
  export IMG=your-image-repo/ipfs-operator:version
  make docker-build
  make docker-push
  make install


Creating a pull request

Pull requests are welcome and encouraged. Please make pull reqeusts against https://github.com/redhat-et/ipfs-operator.
